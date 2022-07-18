#!/usr/bin/env bash
# ISUCON開始時に最初に起動するスクリプト
# ISUCON用のリポジトリのトップレベルに最初から配置されていることを想定

APP_SERVERS=(
    isucon10-qualify-app1
    isucon10-qualify-app2
    isucon10-qualify-app3
)

GITHUB_ACCOUNTS=(
    wataruiwabuchi
)

BACKUP_DIR=/var/backup
MYSQL_DUMP_PATH=${BACKUP_DIR}/backup.dump
BACKUP_TARGETS=(
    ${HOME}
    /etc
    ${BACKUP_DUMP_PATH}
)

source ./deployed_file_paths.sh

# 初期バックアップの取得
sudo mkdir -p ${BACKUP_DIR} && sudo chown -R isucon:isucon ${BACKUP_DIR}
sudo mysqldump -x --all-databases > ${MYSQL_DUMP_PATH}
sudo tar cvzf ${BACKUP_DIR}/backup.tar.gz ${BACKUP_TARGETS[@]} && sudo chown isucon:isucon ${BACKUP_DIR}/backup.tar.gz

# githubから取得した公開鍵をauthorized_keysに配置
# TODO 他サーバの考慮をどうするか
#      そもそもsshできないとファイルを配置することもできない
#      この項目自体が必要ないかもしれない
#AUTHORIZED_KEYS_PATH=${HOME}/.ssh/authorized_keys
#mkdir -p $( dirname ${AUTHORIZED_KEYS_PATH} )
#test -e ${AUTHORIZED_KEYS_PATH} && rm ${AUTHORIZED_KEYS_PATH}
#touch ${AUTHORIZED_KEYS_PATH}
#chmod 600 ${AUTHORIZED_KEYS_PATH}
#for github_account in ${GITHUB_ACCOUNTS[@]}
#do
#    echo -e "$( curl https://github.com/${github_account}.keys )" >> ${AUTHORIZED_KEYS_PATH}
#done

# 各開発者の作業スペースを作成
for github_account in ${GITHUB_ACCOUNTS[@]}
do
    mkdir -p ${HOME}/workspaces/${github_account}
done

# リポジトリを各サーバに配置
for app_server in ${APP_SERVERS[@]}
do
    if [ $( hostname ) = ${app_server} ]; then continue; fi
    git_top_path=$( git rev-parse --show-toplevel )
    ssh isucon@${app_server} "rm -rf ${git_top_path}"
    scp -r ${git_top_path} isucon@${app_server}:$( dirname ${git_top_path} )
    ssh isucon@${app_server} "cd ${git_top_path}; git reset --hard; git clean -fd"
done

# リポジトリに必要なファイルを配置
for common_path in ${COMMON_PATHS[@]}
do
    sudo cp -a ${common_path} ./
done
for uncommon_path in ${UNCOMMON_PATHS[@]}
do
    for app_server in ${APP_SERVERS[@]}
    do
        sudo cp -a ${uncommon_path} ./$( basename ${uncommon_path} ).${app_server}
    done
done

# 必要なツールをインストール
for app_server in ${APP_SERVERS[@]}
do
    ssh isucon@${app_server} 'sudo apt-get install -y glances fish'
done

# デフォルトシェルの変更
for app_server in ${APP_SERVERS[@]}
do
    ssh isucon@${app_server} 'sudo chsh -s /usr/bin/fish isucon'
done

# vimrcとtmux.confの配置
wget -P $HOME https://raw.githubusercontent.com/wataruiwabuchi/vim_config/master/.vimrc -O $HOME/.vimrc
wget -P $HOME https://raw.githubusercontent.com/wataruiwabuchi/tmux_config/master/.tmux.conf -O $HOME/.tmux.conf
