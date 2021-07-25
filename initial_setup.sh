#!/bin/bash
set -eux
# ISUCON開始時に最初に起動するスクリプト
# ISUCON用のリポジトリのトップレベルに最初から配置されていることを想定

APP_SERVERS=(
    localhost
)

GITHUB_ACCOUNTS=(
    wataruiwabuchi
)

# 複数台構成において中身が共通しているもの
COMMON_PATHS=(
    /etc/nginx/nginx.conf
    /etc/mysql/mysql.conf.d/mysqld.cnf
    /home/isucon/isubata/webapp/python/app.py
)
# 複数台構成においてサーバごとに別のファイルにする可能性があるもの
UNCOMMON_PATHS=(
    /etc/nginx/sites-available/nginx.conf
    $HOME/env.sh
)

BACKUP_DIR=/var/backup
BACKUP_TARGETS=(
    ${HOME}
    /etc
)

# 初期バックアップの取得
sudo mkdir -p ${BACKUP_DIR} && sudo chown -R isucon:isucon ${BACKUP_DIR}
sudo tar cvzf ${BACKUP_DIR}/backup.tar.gz ${BACKUP_TARGETS[@]} && sudo chown isucon:isucon ${BACKUP_DIR}/backup.tar.gz
sudo mysqldump -x --all-databases > ${BACKUP_DIR}/backup.dump

# githubから取得した公開鍵をauthorized_keysに配置
# TODO 他サーバの考慮をどうするか
#      そもそもsshできないとファイルを配置することもできない
#      この項目自体が必要ないかもしれない
AUTHORIZED_KEYS_PATH=${HOME}/.ssh/authorized_keys
rm ${AUTHORIZED_KEYS_PATH} && touch ${AUTHORIZED_KEYS_PATH}
chmod 600 ${AUTHORIZED_KEYS_PATH}
for github_account in ${GITHUB_ACCOUNTS[@]}
do
    echo -e "$( curl https://github.com/${github_account}.keys )" >> ${AUTHORIZED_KEYS_PATH}
done

# 各開発者の作業スペースを作成
for github_account in ${GITHUB_ACCOUNTS[@]}
do
    mkdir -p ${HOME}/workspaces/${github_account}
done

# リポジトリに必要なファイルを配置
for common_path in ${COMMON_PATHS[@]}
do
    sudo cp -a ${common_path} ./
done
for uncommon_path in ${UNCOMMON_PATHS[@]}
do
    for app_server in ${APP_SERVERS}
    do
        sudo cp -a ${uncommon_path} ./$( basename ${uncommon_path} ).${app_server}
    done
done

# vimrcとtmux.confの配置
wget -P $HOME https://raw.githubusercontent.com/wataruiwabuchi/vim_config/master/.vimrc -O $HOME/.vimrc
wget -P $HOME https://raw.githubusercontent.com/wataruiwabuchi/tmux_config/master/.tmux.conf -O $HOME/.tmux.conf
