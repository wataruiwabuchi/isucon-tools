#!/bin/bash
set -eux

# デプロイされるファイルのパスを記載

# 複数台構成において中身が共通しているもの
COMMON_PATHS=(
    /etc/nginx/nginx.conf
    /etc/mysql/mysql.conf.d/mysqld.cnf
    /home/isucon/isucari/webapp/python/app.py
)
# 複数台構成においてサーバごとに別のファイルにする可能性があるもの
UNCOMMON_PATHS=(
    /etc/nginx/sites-available/isucari.conf
    $HOME/env.sh
)
