#!/usr/bin/env bash
set -eux

APP_SERVERS=(
    isucon11-qualify-standalone-1
)

GITHUB_ACCOUNTS=(
    wataruiwabuchi
)

# 複数台構成において中身が共通しているもの
COMMON_PATHS=(
    /etc/nginx/nginx.conf
    /etc/mysql/mariadb.conf.d/50-server.cnf
    /home/isucon/webapp/go/main.go
)

# 複数台構成においてサーバごとに別のファイルにする可能性があるもの
UNCOMMON_PATHS=(
    /etc/nginx/sites-available/isucondition.conf
    $HOME/env.sh
)

LOG_FILES=(
    /var/log/mysql/mysql-slow.log
    /var/log/nginx/access.log
    /var/log/nginx/error.log
)

SERVICES=(
    nginx
    mysql
    isucondition.go
)

APP_DIR=$HOME/webapp/go
