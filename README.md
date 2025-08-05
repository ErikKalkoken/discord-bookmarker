# Bookmarks

A Discord bot for bookmarking messages with reminders.

[![Go](https://github.com/ErikKalkoken/discord-bookmarker/actions/workflows/go.yml/badge.svg)](https://github.com/ErikKalkoken/discord-bookmarker/actions/workflows/go.yml)

> [!NOTE]
> The project is currently in active development and not yet ready for production use.

## Installation Guide (WIP)

This section explains how to install **bookmarker** as a service on a Unix-like server.

### Discord App creation

Create a Discord app with the following settings:

- General Information
  - Name: bookmarker
  - Description: A Discord bot for bookmarking messages with reminders.

- Installation
  - Installation Context: User Install only
  - Default Install Settings: `application.commands`

- Bot
  - Create token
  - Public Bot: yes
  - Message Content Intend: yes (Needed to show the content of a bookmarked message)

### Service installation

> [!NOTE]
> This guide uses [supervisor](http://supervisord.org/index.html) for running bookmarker as a service. Please make sure it is installed on your system before continuing.

Create a "service" user with disabled login:

```sh
sudo adduser --disabled-login bookmarker
```

Switch to the service user and move to the home directory:

```sh
sudo su bookmarker
cd ~
```

Download and decompress the latest release from the [releases page](https://github.com/ErikKalkoken/bookmarker/releases):

```sh
wget https://github.com/ErikKalkoken/bookmarker/releases/download/vX.Y.Z/bookmarker-X.Y.Z-linux-amd64.tar.gz
tar -xvzf bookmarker-X.Y.Z-linux-amd64.tar.gz
```

> [!TIP]
> Please make sure update the URL and filename to the latest version.

Download configuration files:

```sh
wget https://raw.githubusercontent.com/ErikKalkoken/bookmarker/main/config/supervisor.conf
```

Setup and configure:

```sh
chmod +x bookmarker
touch bookmarker.log
```

Add your app ID and bot token to the supervisor.conf file.

Add bookmarker to supervisor:

```sh
sudo ln -s /home/bookmarker/supervisor.conf /etc/supervisor/conf.d/bookmarker.conf
sudo systemctl restart supervisor
```

Restart the bookmarker service.

```sh
sudo supervisorctl restart bookmarker
```

## Credits

- Icons: [Bookmark icons created by inkubators - Flaticon](https://www.flaticon.com/free-icons/bookmark)
