#!/bin/sh
# Порт 8080 (monitor-bot) доступен только с сервера метрик и локально
ALLOWED="33.130.151.210 31.130.151.210"
iptables -D INPUT -p tcp --dport 8080 -j MONITOR8080 2>/dev/null || true
iptables -F MONITOR8080 2>/dev/null || iptables -N MONITOR8080
iptables -A MONITOR8080 -i lo -j ACCEPT
for ip in $ALLOWED; do iptables -A MONITOR8080 -s $ip -j ACCEPT; done
iptables -A MONITOR8080 -j DROP
iptables -I INPUT -p tcp --dport 8080 -j MONITOR8080
ip6tables -D INPUT -p tcp --dport 8080 -j MONITOR8080 2>/dev/null || true
ip6tables -F MONITOR8080 2>/dev/null || ip6tables -N MONITOR8080
ip6tables -A MONITOR8080 -i lo -j ACCEPT
ip6tables -A MONITOR8080 -j DROP
ip6tables -I INPUT -p tcp --dport 8080 -j MONITOR8080
