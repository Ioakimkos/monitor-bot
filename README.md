# monitor-bot

Telegram-бот мониторинга (Go). Принимает метрики от удалённого сервера по HTTP (`:8080`), шлёт в группу алерты о падении/восстановлении контейнеров и отдаёт логи контейнеров.

Команды: `/start`, `/status`, `/logs <контейнер>` (только в группе мониторинга).

## Установка

```bash
cd /opt/monitor-bot
go build -o monitor-bot .

# токен бота
install -m 600 deploy/monitor-bot.env.example /etc/monitor-bot.env
nano /etc/monitor-bot.env

# systemd
cp deploy/monitor-bot.service deploy/monitor-bot-firewall.service /etc/systemd/system/
mkdir -p /etc/systemd/system/monitor-bot.service.d
cp deploy/monitor-bot.service.d-env.conf /etc/systemd/system/monitor-bot.service.d/env.conf
install -m 755 deploy/monitor-bot-firewall.sh /usr/local/sbin/monitor-bot-firewall.sh
systemctl daemon-reload
systemctl enable --now monitor-bot-firewall monitor-bot
```

`monitor-bot-firewall.sh` открывает порт 8080 только для сервера с метриками (`ALLOWED`) и localhost.
