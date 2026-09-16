# -*- coding: utf-8 -*-
import argparse
import enum
import re
import sys
import threading
import time
import traceback
from dataclasses import dataclass
from typing import Any
from tqdm import tqdm
from api.answer import Tiku
from api.base import Chaoxing, Account, StudyResult
from api.exceptions import LoginError, InputFormatError
from api.configfile import read_config_file
from api.guard import check_before_run, hard_stop, UserAbort
from api import interrupt
from api.display import ChapterProgress, course_plan_summary, safe_console
from api.logger import set_quiet as set_console_quiet
from api.logger import log_file_only, logger
from requests import RequestException
from api.notification import Notification
from api.live import Live
from api.live_process import LiveProcessor
from api.process import increase_learning_count_for_course

try:
    from queue import PriorityQueue, ShutDown
except ImportError:
    from queue import PriorityQueue


    class ShutDown(Exception):
        pass


class ChapterResult(enum.Enum):
    SUCCESS = 0,
    ERROR = 1,
    NOT_OPEN = 2,
    PENDING = 3


def log_error(func):
    def wrapper(*args, **kwargs):
        try:
            func(*args, **kwargs)
        except BaseException as e:
            logger.error(f"Error in thread {threading.current_thread().name}: {e}")
            traceback.print_exception(type(e), e, e.__traceback__)
            raise

    return wrapper


class NetworkRetryFailed(Exception):
    """网络问题重试多次仍然失败（不是用户配置错误）"""


def with_network_retry(func, *args, what="请求", times=3, delay=2.0, **kwargs):
    """
    主流程的网络请求重试。

    登录 / 取课表 / 取章节这几步以前一次网络抖动就整轮崩掉（#124 #166 #192 #226 #228），
    这里对连接类异常重试几次；仍然失败就抛 NetworkRetryFailed，
    由 main() 统一给一句人话提示，而不是甩一堆 traceback。
    """
    last_error = None
    for attempt in range(1, times + 1):
        try:
            return func(*args, **kwargs)
        except RequestException as e:
            last_error = e
            if attempt < times:
                logger.warning(f"{what}失败（第 {attempt}/{times} 次）：{e}，{int(delay)} 秒后重试")
                time.sleep(delay)
    raise NetworkRetryFailed(f"{what}失败：{last_error}")


def str_to_bool(value):
    if isinstance(value, bool):
        return value
    return str(value).strip().lower() in {"1", "true", "yes", "y", "on"}


def safe_float(value, default, low=None, high=None):
    """
    安全地把配置值转成浮点数。
    配置里写错（"abc"）或超范围时用默认值，绝不让程序崩溃。
    """
    try:
        num = float(str(value).strip())
    except (TypeError, ValueError):
        return default
    if low is not None and num < low:
        return low
    if high is not None and num > high:
        return high
    return num


def safe_int(value, default, low=None, high=None):
    """安全地把配置值转成整数；写错或超范围时用默认值"""
    try:
        num = int(float(str(value).strip()))
    except (TypeError, ValueError):
        return default
    if low is not None and num < low:
        return low
    if high is not None and num > high:
        return high
    return num


NOTOPEN_ACTIONS = ("retry", "ask", "continue")


def parse_args():
    """解析命令行参数"""
    parser = argparse.ArgumentParser(
        description="Samueli924/chaoxing",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )

    parser.add_argument("--use-cookies", action="store_true", help="使用cookies登录")

    parser.add_argument(
        "--yes", "-y", action="store_true",
        help="跳过启动前的确认（用于已确认过的自动化/定时任务；默认必须人工确认）"
    )

    parser.add_argument(
        "-c", "--config", type=str, default=None, help="使用配置文件运行程序"
    )
    parser.add_argument("-u", "--username", type=str, default=None, help="手机号账号")
    parser.add_argument("-p", "--password", type=str, default=None, help="登录密码")
    parser.add_argument(
        "-l", "--list", type=str, default=None, help="要学习的课程ID列表, 以 , 分隔"
    )
    parser.add_argument(
        "-s", "--speed", type=float, default=1.0, help="视频播放倍速 (默认1, 最大2)"
    )
    parser.add_argument(
        "-j", "--jobs", type=int, default=4, help="同时进行的章节数 (默认4, 如果一个章节有多个任务点，不会限制同时处理任务点的数量)"
    )
    parser.add_argument(
        "-n", "--max-points", type=int, default=0,
        help="每门课最多刷几个任务点(章节), 0 或留空=全部刷完"
    )

    parser.add_argument(
        "-v",
        "--verbose",
        "--debug",
        action="store_true",
        help="启用调试模式, 输出DEBUG级别日志",
    )
    parser.add_argument(
        "-a", "--notopen-action", type=str, default="retry",
        choices=["retry", "ask", "continue"],
        help="遇到关闭任务点时的行为: retry-重试, ask-询问, continue-继续"
    )

    parser.add_argument("--auto-sign", action="store_true", help="自动签到")
    parser.add_argument(
        "--retry-interval", type=float, default=1.0, help="重试等待时间, 单位秒 (默认1.0)"
    )

    parser.add_argument(
        "-lc",
        "--add-learning-count",
        action="store_true",
        help="开启章节学习次数增加模式",
    )
    parser.add_argument(
        "-tc",
        "--target-count",
        type=int,
        default=100,
        help="章节学习次数目标总次数 (默认100)",
    )

    # 在解析之前捕获 -h 的行为
    if len(sys.argv) == 2 and sys.argv[1] in {"-h", "--help"}:
        parser.print_help()
        sys.exit(0)

    return parser.parse_args()


def load_config_from_file(config_path):
    """从配置文件加载设置"""
    config, broken = read_config_file(config_path)
    if broken:
        logger.warning(f"配置文件 {config_path} 内容有损坏，已跳过异常行并尽量沿用其余设置")

    common_config: dict[str, Any] = {}
    tiku_config: dict[str, Any] = {}
    notification_config: dict[str, Any] = {}

    # 检查并读取common节
    if config.has_section("common"):
        common_config = dict(config.items("common"))

        # 处理course_list，将字符串转换为列表
        if "course_list" in common_config and common_config["course_list"]:
            common_config["course_list"] = [item.strip() for item in common_config["course_list"].split(",") if
                                            item.strip()]

        # 下面这些数值都用 safe_* 解析：
        # 用户在配置里写错（比如 speed=abc）时用默认值兜底，而不是让程序崩溃。
        if "speed" in common_config:
            common_config["speed"] = safe_float(common_config["speed"], 1.0, low=1.0, high=2.0)
        if "jobs" in common_config:
            common_config["jobs"] = safe_int(common_config["jobs"], 4, low=1, high=8)
        if "retry_interval" in common_config:
            common_config["retry_interval"] = safe_float(common_config["retry_interval"], 1.0, low=0.0)
        else:
            common_config["retry_interval"] = 1.0
        if "work_max_retries" in common_config:
            common_config["work_max_retries"] = safe_int(common_config["work_max_retries"], 3, low=0, high=10)
        else:
            common_config["work_max_retries"] = 3
        if "use_cookies" in common_config:
            common_config["use_cookies"] = str_to_bool(common_config["use_cookies"])
        if "add_learning_count" in common_config:
            common_config["add_learning_count"] = str_to_bool(common_config["add_learning_count"])
        if "target_count" in common_config:
            common_config["target_count"] = safe_int(common_config["target_count"], 100, low=1, high=9999)

        # notopen_action 只接受三个合法值，写错就用默认的 continue（最省事）
        action = str(common_config.get("notopen_action", "")).strip().lower()
        common_config["notopen_action"] = action if action in NOTOPEN_ACTIONS else "continue"
        # max_points_per_course 支持两种格式，必须保持字符串原样交给
        # _parse_max_points 解析：
        #   3                    全部课程都刷 3 个
        #   2151141:3,189191:0   按课程分别指定
        # 注意：不要在这里做 int() 转换，否则 "2151141:3" 会抛 ValueError
        # 被静默改成 0，导致"按课程指定"完全失效、退化成全部刷完。
        if "username" in common_config and common_config["username"] is not None:
            common_config["username"] = common_config["username"].strip()
        if "password" in common_config and common_config["password"] is not None:
            common_config["password"] = common_config["password"].strip()

    # 检查并读取tiku节
    if config.has_section("tiku"):
        tiku_config = dict(config.items("tiku"))
        # 处理数值类型转换（写错时用默认值兜底，不让程序崩溃）
        for key, default in (("delay", 1.0), ("cover_rate", 0.8)):
            if key in tiku_config:
                tiku_config[key] = safe_float(tiku_config[key], default, low=0.0)

    # 检查并读取notification节
    if config.has_section("notification"):
        notification_config = dict(config.items("notification"))

    return common_config, tiku_config, notification_config


def build_config_from_args(args):
    """从命令行参数构建配置"""
    common_config = {
        "use_cookies": args.use_cookies,
        "username": args.username,
        "password": args.password,
        "course_list": [item.strip() for item in args.list.split(",") if item.strip()] if args.list else None,
        "speed": args.speed or 1.0,
        "jobs": args.jobs,
        "notopen_action": args.notopen_action or "retry",
        "retry_interval": args.retry_interval or 1.0,
        "add_learning_count": args.add_learning_count,
        "target_count": args.target_count,
        "max_points_per_course": getattr(args, "max_points", 0) or 0,
    }
    return common_config, {}, {}


def init_config():
    """初始化配置"""
    args = parse_args()

    if args.config:
        common_config, tiku_config, notification_config = load_config_from_file(args.config)
    else:
        common_config, tiku_config, notification_config = build_config_from_args(args)
    return common_config, tiku_config, notification_config, args.config, args


def init_chaoxing(common_config, tiku_config, config_path=None):
    """初始化超星实例"""
    username = common_config.get("username", "")
    password = common_config.get("password", "")
    use_cookies = common_config.get("use_cookies", False)

    # 如果没有提供用户名密码，从命令行获取
    if (not username or not password) and not use_cookies:
        username = input("请输入你的手机号, 按回车确认\n手机号:")
        password = input("请输入你的密码, 按回车确认\n密码:")

    account = Account(username, password)

    # 设置题库
    tiku = Tiku.get_tiku_from_config(tiku_config, config_path=config_path)  # 载入题库
    tiku.init_tiku()  # 初始化题库

    # 获取查询延迟设置

    # 检查大模型连接（如果使用的是大模型题库）
    # 根据配置文件中的 provider 判断是否为大模型题库
    provider = tiku_config.get('provider', '')
    provider_list = [name.strip() for name in provider.split(',') if name.strip()]
    if any(name in ['AI', 'SiliconFlow'] for name in provider_list):
        check_connection = tiku_config.get('check_llm_connection', 'true').lower() == 'true'
        if check_connection:
            logger.info(f'正在验证大模型配置 (provider={provider})...')
            if not tiku.check_llm_connection():
                logger.error('大模型连接检查失败')

                # 没有终端可交互时，不能自己决定继续，直接停止
                if not sys.stdin.isatty():
                    raise RuntimeError(
                        'DeepSeek API Key 校验失败，且当前无法交互确认，已停止运行。\n'
                        '        请检查 config.ini 里的 key（或运行 cx setup 重新填写）。'
                    )

                print()
                print("  ✘ API Key 校验失败，章节测验将无法作答")
                print("    可能原因：填错了、已失效、或账户余额不足。")
                print("    建议先运行 cx 重新填写；这里选停止更安全。")
                print()
                try:
                    choice = input("  仍然继续刷课吗？(y/n) > ").strip().lower()
                except (EOFError, KeyboardInterrupt):
                    print()
                    choice = ""
                # 默认不继续（安全默认），必须明确输入 y
                if choice not in ("y", "yes", "是"):
                    raise RuntimeError("API Key 校验失败，用户选择停止")
                logger.warning("用户确认在 API Key 异常的情况下继续运行")

    query_delay = tiku_config.get("delay", 0)

    # 章节检测答错后允许的最大重做次数（答错时反馈给AI并重新提交，直到全部正确）
    work_max_retries = common_config.get("work_max_retries", 3)

    # 视频是否串行：默认并发（快）；遇到"刷完又变回没刷"可以打开
    serial_video = str_to_bool(common_config.get("serial_video", False))

    # 实例化超星API
    chaoxing = Chaoxing(
        account=account,
        tiku=tiku,
        query_delay=query_delay,
        work_max_retries=work_max_retries,
        serial_video=serial_video,
    )

    return chaoxing


def process_job(chaoxing: Chaoxing, course: dict, job: dict, job_info: dict, speed: float) -> StudyResult:
    """处理单个任务点"""
    # 视频任务
    if job["type"] == "video":
        logger.trace(f"识别到视频任务, 任务章节: {course['title']} 任务ID: {job['jobid']}")
        # 超星的接口没有返回当前任务是否为Audio音频任务
        video_result = chaoxing.study_video(
            course, job, job_info, _speed=speed, _type="Video"
        )
        if video_result.is_failure():
            # 音频任务在超星接口里也标记为 video，这是常见的正常回退
            logger.info("当前任务非视频任务, 正在尝试音频任务解码")
            video_result = chaoxing.study_video(
                course, job, job_info, _speed=speed, _type="Audio")
        if video_result.is_failure():
            logger.warning(
                f"出现异常任务 -> 任务章节: {course['title']} 任务ID: {job['jobid']}, 已跳过"
            )
        return video_result
    # 文档任务
    elif job["type"] == "document":
        logger.trace(f"识别到文档任务, 任务章节: {course['title']} 任务ID: {job['jobid']}")
        return chaoxing.study_document(course, job)
    # 测验任务
    elif job["type"] == "workid":
        logger.trace(f"识别到章节检测任务, 任务章节: {course['title']}")
        return chaoxing.study_work(course, job, job_info)
    # 阅读任务
    elif job["type"] == "read":
        logger.trace(f"识别到阅读任务, 任务章节: {course['title']}")
        return chaoxing.study_read(course, job, job_info)
    # 直播任务
    elif job["type"] == "live":
        logger.trace(f"识别到直播任务, 任务章节: {course['title']} 任务ID: {job['jobid']}")
        try:
            # 准备直播所需参数
            defaults = {
                "userid": chaoxing.get_uid(),
                "clazzId": course.get("clazzId"),
                "knowledgeid": job_info.get("knowledgeid")
            }

            # 创建直播对象
            live = Live(
                attachment=job,
                defaults=defaults,
                course_id=course.get("courseId")
            )

            # 启动直播处理线程
            thread = threading.Thread(
                target=LiveProcessor.run_live,
                args=(live, speed),
                daemon=True
            )
            thread.start()
            thread.join()  # 等待直播处理完成
            return StudyResult.SUCCESS
        except Exception as e:
            logger.error(f"处理直播任务时出错: {str(e)}")
            return StudyResult.ERROR

    logger.error(f"未知任务类型: {job['type']}")
    return StudyResult.ERROR


@dataclass
class ChapterTask:
    index: int
    point: dict[str, Any]
    course: dict[str, Any]
    result: ChapterResult = ChapterResult.PENDING
    tries: int = 0

    def __lt__(self, other):
        """比较两个任务的索引大小，用于优先级队列排序."""
        if not isinstance(other, ChapterTask):
            return NotImplemented
        return self.index < other.index


class JobProcessor:
    def __init__(self, chaoxing: Chaoxing, tasks: list[ChapterTask], config: dict[str, Any],
                 progress=None):
        """初始化任务处理器."""
        if "jobs" not in config or not config["jobs"]:
            config["jobs"] = 4

        self.chaoxing = chaoxing
        self.speed = config["speed"]
        self.max_tries = 5
        self.tasks = tasks
        self.failed_tasks: list[ChapterTask] = []
        self.task_queue: PriorityQueue[ChapterTask] = PriorityQueue()
        self.retry_queue: PriorityQueue[ChapterTask] = PriorityQueue()
        self.wait_queue: PriorityQueue[ChapterTask] = PriorityQueue()
        self.threads: list[threading.Thread] = []
        self.worker_num = config["jobs"]
        self.config = config
        self.retry_interval = config.get("retry_interval", 1.0)
        self.progress = progress

    def run(self):
        for task in self.tasks:
            self.task_queue.put(task)

        for i in range(self.worker_num):
            thread = threading.Thread(target=self.worker_thread, daemon=True)
            self.threads.append(thread)
            thread.start()

        threading.Thread(target=self.retry_thread, daemon=True).start()

        # 等待所有任务完成。
        # 不用 task_queue.join()：如果工作线程意外全部退出，join() 会永久挂起
        # （表现为程序变成僵尸进程，既不报错也不退出）。这里加看门狗。
        while True:
            try:
                if self.task_queue.unfinished_tasks == 0:
                    break
            except AttributeError:
                break
            if interrupt.should_stop():
                logger.warning("收到终止指令，停止等待剩余任务")
                break
            alive = [t for t in self.threads if t.is_alive()]
            if not alive:
                remaining = getattr(self.task_queue, "unfinished_tasks", 0)
                logger.error(
                    "所有工作线程已退出，但仍有 {} 个任务未完成，停止等待", remaining
                )
                break
            time.sleep(0.3)

        time.sleep(0.5)
        if hasattr(self.task_queue, "shutdown"):
            self.task_queue.shutdown()

    @log_error
    def worker_thread(self):
        while True:
            # 用户按 q 要求终止 -> 立刻停止领取新任务
            if interrupt.should_stop():
                return
            try:
                task = self.task_queue.get()
            except ShutDown:
                logger.info("Queue shut down")
                return

            # 关键：单个章节的异常不能让线程退出，否则任务永远不会 task_done，
            # 主线程会永久等待。异常统一转成 ERROR，交给下面的重试逻辑处理。
            try:
                task.result = process_chapter(self.chaoxing, task.course, task.point, self.speed)
            except BaseException as e:
                logger.error(
                    "处理章节时发生异常: {} - {} -> {}: {}",
                    task.course.get("title", "?"), task.point.get("title", "?"),
                    type(e).__name__, e
                )
                logger.debug(traceback.format_exc())
                task.result = ChapterResult.ERROR

            match task.result:
                case ChapterResult.SUCCESS:
                    logger.debug("Task success: {} - {}", task.course["title"], task.point["title"])
                    if self.progress:
                        self.progress.chapter_done(task.point.get("title", ""))
                    self.task_queue.task_done()
                    logger.debug(f"unfinished task: {self.task_queue.unfinished_tasks}")

                case ChapterResult.NOT_OPEN:
                    if self.config["notopen_action"] == "continue":
                        # 进度行已用 ⤼ 标记，这里只写日志文件，不刷屏
                        logger.info("章节未开启，已跳过: {} - {}", task.course["title"], task.point["title"])
                        if self.progress:
                            self.progress.chapter_skipped(task.point.get("title", ""), "未开放")
                        self.task_queue.task_done()
                        continue

                    task.tries += 1
                    if task.tries >= self.max_tries:
                        logger.info(
                            "章节未开启(重试已达上限): {} - {} 可能由于上一章节的章节检测未完成, "
                            "或该章节因时效已关闭，请手动检查完成并提交再重试。"
                            , task.course["title"], task.point["title"])
                        if self.progress:
                            self.progress.chapter_skipped(task.point.get("title", ""), "(未开放)")
                        self.task_queue.task_done()
                        continue

                    # self.wait_queue.put(task)
                    self.retry_queue.put(task)

                case ChapterResult.ERROR:
                    task.tries += 1
                    # 重试过程写日志文件即可，控制台由进度行体现
                    logger.info("重试任务 {} - {} ({}/{} 次尝试)", task.course["title"], task.point["title"],
                                task.tries,
                                self.max_tries)
                    if task.tries >= self.max_tries:
                        # 进度行已用 ✗ 标记
                        logger.info("任务重试次数达到上限: {} - {}", task.course["title"], task.point["title"])
                        self.failed_tasks.append(task)
                        if self.progress:
                            self.progress.chapter_failed(task.point.get("title", ""))
                        self.task_queue.task_done()
                        continue
                    self.retry_queue.put(task)

                case _:
                    logger.error("任务 {} 的状态无效 {}", task.result, task.point["title"])
                    self.failed_tasks.append(task)
                    self.task_queue.task_done()

    @log_error
    def retry_thread(self):
        try:
            while True:
                task = self.retry_queue.get()
                self.task_queue.put(task)
                # task_done is not called when a task failed and needs to be retried so if is reinserted into the queue,
                # the task num will increase by one and become more than the real task number
                self.task_queue.task_done()
                time.sleep(self.retry_interval)
        except ShutDown:
            pass


def process_chapter(chaoxing: Chaoxing, course: dict[str, Any], point: dict[str, Any], speed: float) -> ChapterResult:
    """处理单个章节"""
    # 用户已要求终止：不再开始新章节
    if interrupt.should_stop():
        return ChapterResult.ERROR
    logger.info(f'当前章节: {point["title"]}')
    if point["has_finished"]:
        logger.info(f'章节：{point["title"]} 已完成所有任务点')
        return ChapterResult.SUCCESS

    # 随机等待，避免请求过快
    chaoxing.rate_limiter.limit_rate(random_time=True, random_min=0, random_max=0.2)

    # 获取当前章节的所有任务点
    job_info = None
    jobs, job_info = chaoxing.get_job_list(course, point)

    # 发现未开放章节, 根据配置处理
    if job_info.get("notOpen", False):
        return ChapterResult.NOT_OPEN

    # 已经默认处理空任务，此处不需要判断
    if not jobs:
        pass

    job_results: list[StudyResult] = []
    for job in jobs:
        result = process_job(chaoxing, course, job, job_info, speed)
        job_results.append(result)

    for result in job_results:
        if result.is_failure():
            return ChapterResult.ERROR

    return ChapterResult.SUCCESS


def process_course(chaoxing: Chaoxing, course: dict[str, Any], config: dict):
    """处理单个课程"""
    logger.info(f"开始学习课程: {course['title']}")

    # 获取当前课程的所有章节
    point_list = chaoxing.get_course_point(
        course["courseId"], course["clazzId"], course["cpi"]
    )

    # 为了支持课程任务回滚, 采用下标方式遍历任务点

    _old_format_sizeof = tqdm.format_sizeof
    tqdm.format_sizeof = format_time

    tasks = []

    for i, point in enumerate(point_list["points"]):
        task = ChapterTask(point=point, index=i, course=course)
        tasks.append(task)
    p = JobProcessor(chaoxing, tasks, config)
    p.run()

    tqdm.format_sizeof = _old_format_sizeof


def _parse_max_points(raw):
    """
    解析 max_points_per_course 配置。

    支持：
      3                   -> ({}, 3, [])            全部课程都刷 3 个
      2151141:3,189191:0  -> ({'2151141':3, ...}, 0, [])
      0 或空              -> ({}, 0, [])            全部刷完
    返回 (每课程字典, 默认值, 无法识别的片段)

    第三个返回值很重要：如果用户填了东西却一个都认不出来，
    必须停下来提醒，绝不能默默当成"全部刷完"。
    """
    if raw is None:
        return {}, 0, []
    text = str(raw).strip()
    if not text:
        return {}, 0, []

    per_course = {}
    default = None
    bad = []

    for item in re.split(r"[，,、;；\s]+", text):
        item = item.strip()
        if not item:
            continue
        if ":" in item or "：" in item:
            # 课程ID:数量
            parts = re.split(r"[:：]", item, maxsplit=1)
            cid = parts[0].strip()
            try:
                num = int(parts[1].strip())
            except (ValueError, IndexError):
                bad.append(item)
                continue
            if not cid:
                bad.append(item)
                continue
            per_course[cid] = max(0, num)
        else:
            # 单个数字 = 全局默认
            try:
                default = max(0, int(item))
            except ValueError:
                bad.append(item)
                continue

    if default is None:
        default = 0
    return per_course, default, bad


def select_points_for_course(all_points, max_points=0):
    """
    把一门课的章节分成"已完成"和"待刷"两部分，并算出本次要刷哪些。

    已完成（has_finished）的章节直接跳过，不再排进任务队列 ——
    否则每节都会闪过一行"预计 1 秒"，看起来像是要把前面几章重刷一遍。

    返回 (已完成, 待刷, 本次要刷)
    """
    all_points = list(all_points or [])
    finished = [p for p in all_points if p.get("has_finished")]
    pending = [p for p in all_points if not p.get("has_finished")]
    if max_points and max_points > 0:
        selected = pending[:max_points]
    else:
        selected = pending
    return finished, pending, selected


def _format_course_table(all_course):
    """格式化课程列表，供用户选择或报错时展示"""
    lines = ["*" * 10 + "课程列表" + "*" * 10]
    for course in all_course:
        lines.append(f"ID: {course['courseId']} 班级ID: {course['clazzId']} 课程名: {course['title']}")
    lines.append("*" * 28)
    return "\n".join(lines)


def _parse_course_ids(raw):
    """
    解析用户输入的课程ID，兼容中文逗号 / 空格 / 换行 / 全角数字等常见误输入。
    返回去重后的 ID 列表。
    """
    if raw is None:
        return []
    # 已经是列表（例如配置里解析后的 course_list）则逐个处理，避免 str(list) 变成 "['111']"
    if isinstance(raw, (list, tuple, set)):
        candidates = [str(x) for x in raw]
    else:
        # 中文逗号、顿号、分号、空格、换行统一成英文逗号
        candidates = re.sub(r"[，、；;\s]+", ",", str(raw).strip()).split(",")
    parts = []
    for item in candidates:
        item = item.strip().strip('"').strip("'")
        # 全角数字转半角
        item = item.translate(str.maketrans("０１２３４５６７８９", "0123456789"))
        if item:
            parts.append(item)
    return list(dict.fromkeys(parts))


def filter_courses(all_course, course_list):
    """
    过滤要学习的课程。

    规则（严格模式）：
      必须明确指定要刷的课程 ID。
        - 匹配成功   -> 只刷这些课程
        - 一个都没匹配上 -> 报错停止，绝不回退全刷
        - 没有指定(空)   -> 报错停止，绝不自动刷全部课程

    历史上这里有个"没指定就刷全部课程"的兜底逻辑，曾导致用户只想刷 1 门课
    却把 12 门课全部刷了。该兜底已移除。
    """
    if not all_course:
        raise InputFormatError("登录成功但没读到任何课程，请检查账号是否有课程")

    # 没有配置课程 ID：打印课程表并停止，由用户明确指定
    wanted = _parse_course_ids(course_list)
    if not wanted:
        raise InputFormatError(
            "没有指定要刷的课程 ID，为避免误刷全部课程已停止运行。\n"
            "请把你想要刷的课程 ID 填到 config.ini 的 course_list，或运行 cx setup。\n"
            + _format_course_table(all_course)
        )

    course_task = []
    seen_keys = set()
    matched_ids = set()
    for course in all_course:
        key = (course["courseId"], course["clazzId"])
        if str(course["courseId"]) in wanted and key not in seen_keys:
            course_task.append(course)
            seen_keys.add(key)
            matched_ids.add(str(course["courseId"]))

    if not course_task:
        raise InputFormatError(
            "配置的 course_list 没有匹配到任何课程，为避免误刷已停止运行。\n"
            f"你填写的: {', '.join(wanted)}\n"
            "请从下面的课程列表里复制正确的 ID 后重试:\n"
            + _format_course_table(all_course)
        )

    missing = [cid for cid in wanted if cid not in matched_ids]
    if missing:
        logger.warning(
            "以下课程ID未匹配到任何课程, 已忽略: {}", ", ".join(missing)
        )

    return course_task


def format_time(num, suffix='', divisor=''):
    total_time = round(num)
    sec = total_time % 60
    mins = (total_time % 3600) // 60
    hrs = total_time // 3600

    if hrs > 0:
        return f"{hrs:02d}:{mins:02d}:{sec:02d}"

    return f"{mins:02d}:{sec:02d}"


def main():
    """主程序入口"""
    try:
        # 初始化配置
        common_config, tiku_config, notification_config, config_path, args = init_config()

        # 强制播放按照配置文件调节
        common_config["speed"] = min(2.0, max(1.0, common_config.get("speed", 1.0)))
        common_config["notopen_action"] = common_config.get("notopen_action", "retry")
        
        # 初始化增加章节学习次数配置
        add_learning_count = str_to_bool(common_config.get("add_learning_count", False))
        target_count = int(common_config.get("target_count", 100))

        # ===== 启动前人工确认门禁 =====
        # 配置缺失/将要降级运行时，必须人工确认；用户不确认就停止，绝不静默降级
        check_before_run(common_config, tiku_config, notification_config, config_path,
                         skip_confirm=getattr(args, "yes", False))

        # 每门课最多刷几个任务点（0 或空 = 全部）
        # 支持两种格式：
        #   max_points_per_course = 3                    全部课程都刷 3 个
        #   max_points_per_course = 2151141:3,189191:0   按课程分别指定（0=全部）
        # 一个都认不出来时必须停在原地提醒，绝不能默默退化成"全部刷完"。
        _raw_points = common_config.get("max_points_per_course")
        max_points_map, max_points_default, _bad_points = _parse_max_points(_raw_points)
        if _bad_points:
            hard_stop(
                "要刷的任务点数量填错了，无法识别",
                f"  当前填写：{_raw_points}\n"
                f"  认不出的部分：{'、'.join(_bad_points)}\n"
                "  正确写法：3                     → 每门课刷 3 个任务点\n"
                "            2151141:3,189191:0    → 指定课程刷 3 个，189191 刷完全部",
                "  运行 cx 重新选择要刷的任务点数量，或直接修改配置文件",
            )

        # 初始化超星实例
        chaoxing = init_chaoxing(common_config, tiku_config, config_path=config_path)

        # 设置外部通知
        notification = Notification()
        notification.config_set(notification_config)
        notification = notification.get_notification_from_config()
        notification.init_notification()

        # 检查当前登录状态（网络抖动自动重试，不再一次就整轮崩）
        _login_state = with_network_retry(
            chaoxing.login,
            login_with_cookies=common_config.get("use_cookies", False),
            what="登录",
        )
        if not _login_state["status"]:
            raise LoginError(_login_state["msg"])

        # 获取所有的课程列表
        all_course = with_network_retry(chaoxing.get_course_list, what="读取课程列表")

        # 过滤要学习的课程
        course_task = filter_courses(all_course, common_config.get("course_list"))

        # 开始学习
        logger.trace(f"课程列表过滤完毕, 当前课程任务数量: {len(course_task)}")

        _old_format_sizeof = tqdm.format_sizeof
        tqdm.format_sizeof = format_time

        # 任务点数量已在启动检查时解析并校验（见上面的 max_points_map / max_points_default）
        tasks = []
        # 一个章节都没读到的课程：不能当成"已经刷完"，否则解析出问题时会误报"无需刷课"
        unreadable_courses = []
        for course in course_task:
            logger.trace(f"正在读取课程章节: {course['title']}")
            point_list = with_network_retry(
                chaoxing.get_course_point,
                course["courseId"], course["clazzId"], course["cpi"],
                what=f"读取《{course['title']}》的章节",
            )
            all_points = point_list.get("points") or []

            if not all_points:
                unreadable_courses.append(course["title"])
                logger.error("课程[{}] 没有读到任何章节", course["title"])
                print("  ⚠ " + course["title"] + "：没有读到任何章节（可能是页面结构变化或网络异常）")
                continue

            # 已经刷完的章节不再排进任务队列：整段跳过，只给一行汇总说明
            cid = str(course["courseId"])
            max_points = max_points_map.get(cid, max_points_default)
            finished, pending, selected = select_points_for_course(all_points, max_points)

            logger.info(
                "课程[{}] 共 {} 节, 已完成 {} 节, 待刷 {} 节, 本次刷 {} 节",
                course["title"], len(all_points), len(finished), len(pending), len(selected)
            )
            print("  " + course["title"] + "：" + course_plan_summary(finished, pending, len(selected)))

            for i, point in enumerate(selected):
                task = ChapterTask(point=point, index=i, course=course)
                tasks.append(task)

        # 所有课程都已经刷完：不用再走后面的刷课流程，也不用让用户白等
        if not tasks:
            tqdm.format_sizeof = _old_format_sizeof
            print()
            if unreadable_courses:
                # 读不到章节 ≠ 刷完了。宁可报错让人来看，也不能骗用户说"已全部完成"
                print("  ✘ 有课程没能读到章节，无法判断是否已刷完：" + "、".join(unreadable_courses))
                print("  可能是平台页面结构变化或网络异常，请稍后重试；如果一直这样请反馈。")
                print()
                try:
                    notification.send(
                        "超星刷课：读取失败\n以下课程没有读到章节：" + "、".join(unreadable_courses)
                    )
                except Exception:
                    pass
                sys.exit(4)
            print("  ✔ 所有课程的任务点都已刷完，没有需要重刷的内容")
            print()
            try:
                notification.send("超星刷课：无需刷课\n所有课程的任务点都已刷完")
            except Exception:
                pass
            return

        # 刷课开始：提示如何退出，并启动键盘监听（按 q 立即终止）
        interrupt.print_hint()
        interrupt.start_watcher()

        # 记录开始时间 + 通知开始
        run_started_at = time.time()
        total_points = len(tasks)
        course_names = "、".join(c["title"] for c in course_task)
        logger.info(f"开始刷课：{len(course_task)} 门课，共 {total_points} 个任务点")
        notification.send(
            "超星刷课：已开始\n"
            f"课程（{len(course_task)} 门）：{course_names}\n"
            f"任务点：{total_points} 个"
        )

        # 打开控制台静音：刷课期间只显示 WARNING 及以上 + 进度行，
        # 避免 TRACE/DEBUG 刷屏（日志文件仍然记录全量）
        set_console_quiet(True)
        print()
        print(f"  开始刷课：{len(course_task)} 门课，共 {total_points} 个任务点")
        print("  ✓ 完成   ⤼ 跳过   ✗ 失败            按 q 可随时停止")
        print("  " + "─" * 46)
        print()

        progress = ChapterProgress(total_points)

        # 全局并发执行所有课程的任务点
        # 用 try/finally 保证无论成功、中断还是异常，控制台都会恢复非静音
        try:
            p = JobProcessor(chaoxing, tasks, common_config, progress=progress)
            p.run()
        finally:
            progress.summary()
            set_console_quiet(False)

        if interrupt.should_stop():
            logger.warning("刷课已被用户终止")
            print()
            print("  已终止刷课。已完成的任务点会保留，下次运行会自动跳过。")
            print()
            used = int(time.time() - run_started_at)
            try:
                notification.send(
                    "超星刷课：已手动终止\n"
                    f"课程（{len(course_task)} 门）：{course_names}\n"
                    f"任务点：{total_points} 个（未刷完）\n"
                    f"已运行：{used // 60} 分 {used % 60} 秒"
                )
            except Exception:
                pass
            return

        tqdm.format_sizeof = _old_format_sizeof

        used = int(time.time() - run_started_at)
        used_text = (f"{used // 60} 分 {used % 60} 秒" if used >= 60 else f"{used} 秒")

        # 有任务点没刷成功就不能说"全部完成"：否则用户以为已经刷完，
        # 实际上还差几节（#618）。这里按实际失败数量分开报。
        failed_points = getattr(progress, "failed", 0) or 0
        if failed_points or unreadable_courses:
            logger.warning(f"有 {failed_points} 个任务点未能完成")
            print()
            if failed_points:
                print(f"  ⚠ 有 {failed_points} 个任务点未能完成（下次运行会自动重试）")
            if unreadable_courses:
                print("  ⚠ 有课程没能读到章节：" + "、".join(unreadable_courses))
            print(flush=True)
            unreadable_text = ("\n读不到章节的课程：" + "、".join(unreadable_courses)) if unreadable_courses else ""
            notification.send(
                "超星刷课：部分完成（有失败）\n"
                f"课程（{len(course_task)} 门）：{course_names}\n"
                f"任务点：成功 {max(0, total_points - failed_points)} 个 · 失败 {failed_points} 个\n"
                f"耗时：{used_text}"
                f"{unreadable_text}"
            )
        else:
            logger.info("所有课程学习任务已完成")
            notification.send(
                "超星刷课：全部完成\n"
                f"课程（{len(course_task)} 门）：{course_names}\n"
                f"任务点：{total_points} 个\n"
                f"耗时：{used_text}"
            )

        # 刷课完成后，如果开启了增加章节学习次数，则执行
        if add_learning_count:
            logger.info("刷课完成，开始增加章节学习次数...")
            common_config["target_count"] = target_count
            for course in course_task:
                increase_learning_count_for_course(chaoxing, course, common_config)
            logger.info("所有课程章节学习次数增加完成")
            notification.send("超星刷课：章节学习次数已刷完")
        
    except UserAbort as e:
        # 用户未确认/选择停止：不是程序错误，但用非 0 退出码，方便脚本判断"没跑成"
        logger.warning(f"已停止: {e}")
        sys.exit(3)
    except SystemExit as e:
        if e.code != 0:
            logger.error(f"错误: 程序异常退出, 返回码: {e.code}")
        sys.exit(e.code)
    except KeyboardInterrupt as e:
        logger.error(f"错误: 程序被用户手动中断, {e}")
    except (LoginError, InputFormatError, NetworkRetryFailed) as e:
        # 登录失败 / 网络不稳 / 输入格式错，都是用户自己处理一下就能解决的问题。
        # 只给一行清晰提示 + 处理办法，不打印一堆 traceback 吓人。
        log_file_only(str(e))
        print()
        print("  ✘ " + str(e))
        if isinstance(e, NetworkRetryFailed):
            print("  网络不太稳定，请检查网络 / 代理后重试。")
        else:
            print("  请检查手机号 / 密码是否正确，或运行 cx setup 重新配置。")
        print(flush=True)
        try:
            notification.send(f"超星刷课：启动失败\n{e}")
        except Exception:
            pass
        sys.exit(2)
    except BaseException as e:
        logger.error(f"错误: {type(e).__name__}: {e}")
        logger.error(traceback.format_exc())
        try:
            notification.send(
                f"超星刷课：出现错误\n"
                f"{type(e).__name__}: {e}"
            )
        except Exception:
            pass  # 如果通知发送失败，忽略异常
        raise e


if __name__ == "__main__":
    safe_console()
    main()
