# sms2mail

`sms2mail` is a lightweight Go application that acts as a bridge between Twilio SMS webhooks and email. It receives incoming SMS messages via an HTTP endpoint and forwards them to a specified email address using `msmtp`.

## Features

-   **Twilio Integration**: Handles incoming SMS webhooks from Twilio.
-   **Email Forwarding**: Uses `msmtp` to send emails, supporting various SMTP providers (Gmail, SendGrid, etc.).
-   **Flexible Configuration**: Supports configuration via TOML files and CLI arguments.
-   **Cross-Platform**: Builds for macOS, Linux, and Windows.

## Prerequisites

-   **Go**: Version 1.25 or later (for building).
-   **msmtp**: Must be installed and configured on the system.
    -   macOS: `brew install msmtp`
    -   Linux: `sudo apt install msmtp` (or equivalent)
    -   Windows: Download from [msmtp.sourceforge.net](https://msmtp.sourceforge.net/)

Ensure `msmtp` is in your system `PATH` and configured correctly (usually `~/.msmtprc` or `/etc/msmtprc`).

## Installation

1.  Clone the repository.
2.  Build the binary:
    ```bash
    go build -o sms2mail
    ```
    Or use the build script to generate binaries for multiple platforms:
    ```bash
    ./build.sh
    ```

## Configuration

`sms2mail` now supports multiple configuration profiles.

1.  **Global Configuration**: Defines server settings (port).
2.  **Profile Configuration**: Defines email settings for specific endpoints.

### Global Configuration File

Located at `/etc/sms2mail.toml`, `~/.config/sms2mail.toml`, or `./config.toml`.

```toml
# Server settings
server_port = ":8080"
```

### Profile Configuration Files

Located in a `sms2mail.d` directory next to the global config file (e.g., `/etc/sms2mail.d/` or `./sms2mail.d/`).

Example: `sms2mail.d/default.toml`

```toml
# Email Configuration for 'default' profile
email_from = "sms-notifier@yourserver.com"
email_to = "youremail@example.com"
```

You can create multiple profile files (e.g., `marketing.toml`, `alerts.toml`).

## Usage

### Starting the Server

Run the application:

```bash
./sms2mail
```

### Webhook Endpoints

The endpoint URL determines which profile is used:

`http://<server-ip>:8080/sms/<profile-name>`

-   `/sms/default` uses `sms2mail.d/default.toml`
-   `/sms/marketing` uses `sms2mail.d/marketing.toml`

### Generating Configuration

Generate global config template:

```bash
./sms2mail config > config.toml
```

Generate profile config template:

```bash
./sms2mail profileConfig > sms2mail.d/myprofile.toml
```

(Note: You will need to manually create the `sms2mail.d` directory).

## Twilio Setup

1.  Deploy `sms2mail`.
2.  Create a profile (e.g., `sms2mail.d/myphone.toml`).
3.  In Twilio, set the webhook to:
    `http://<your-server-ip>:8080/sms/myphone`

## License

[MIT](LICENSE)
