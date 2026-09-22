# Security Policy

## Supported versions

| Version | Supported |
| --- | --- |
| 3.1.x | ✅ Actively maintained |
| Older | ❌ No longer supported |

## Reporting a vulnerability

Please **do not open a public issue** for a security problem. Report it privately:

1. Open a **private security advisory** on GitHub: repository page → **Security** tab → **Report a vulnerability**.
2. Or contact the maintainer [@songoao25](https://github.com/songoao25) and say you have a security matter to discuss privately.

We will acknowledge the report as soon as possible, assess the severity, and ship a patch release with a note in [CHANGELOG.md](CHANGELOG.md).

## Security commitments

- No user data is collected or uploaded.
- Accounts, cookies and configuration stay on your machine under `~/.chaoxing/` (directory `0700`, credential files `0600`).
- Every capture sample in this repository is sanitized: no cookies, no tokens.
- No hard-coded keys or secrets.
- No access-control bypass. Login only uses the credentials you provide, and login captchas are recognised locally with OCR.

## Please never commit

Cookies, account passwords, API keys, anything under `~/.chaoxing/`, and any capture or log containing a token.
Check before opening an issue or a pull request. If you committed one by accident, change the password and revoke the key immediately.
