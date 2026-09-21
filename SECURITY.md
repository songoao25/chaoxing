# 安全策略（Security Policy）

## 支持的版本

| 版本 | 支持状态 |
| --- | --- |
| 3.1.x | ✅ 积极维护 |
| 更早版本 | ❌ 不再支持 |

## 报告漏洞

如果发现安全漏洞，**请不要公开提交 Issue**，请通过以下方式私下报告：

1. 在 GitHub 上创建一个 **Private security advisory**：仓库主页 → **Security** 标签页 → **Report a vulnerability**；
2. 或联系维护者 [@songoao25](https://github.com/songoao25)，说明「有安全事项需私下沟通」。

我们会尽快确认收到报告、评估严重程度，修复后发布补丁版本并在 [CHANGELOG.md](CHANGELOG.md) 中记录。

## 安全承诺

- 不收集、不上传任何用户数据；
- 账号、cookie、配置只存放在本机 `~/.chaoxing/`（目录 `0700`，凭据文件 `0600`）；
- 仓库内所有抓包样例均已脱敏，不含 cookie 与 token；
- 不包含硬编码密钥或敏感信息；
- 不提供访问控制绕过；登录只使用你提供的凭据，并用本机 OCR 识别登录验证码。

## 请不要提交的内容

Cookie、账号密码、API Key、`~/.chaoxing/` 下的任何文件、含 token 的抓包或日志。
提 Issue 或 PR 前请先检查一遍；如果不慎已提交，请立即修改平台密码并撤销相关 Key。
