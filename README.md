# dokky-telegram-bot

![dokky-telegram-bot](http://i.imgur.com/8KhIOmW.png)

# Install
First, follow install instructions for [go-libreofficekit](https://github.com/docsbox/go-libreofficekit), then type this into your terminal:

```bash
$ apt-get install libmagic-dev
$ go get -u github.com/robotics-notes/dokky-telegram-bot
```

# Usage
```bash
$ export TELEGRAM_BOT_TOKEN="your-bot-token-here"
$ $GOPATH/bin/dokky-telegram-bot
# Will print something like this:
# 2016/09/09 18:29:44 Started.
# 2016/09/09 18:29:44 LibreOfficeKit pool size: 1.
# 2016/09/09 18:29:44 Loaded LibreOfficeKit #1.
# 2016/09/09 18:29:45 Authorized on account DokkyBot.
```
