#!/usr/bin/env bash
set -eux

# 3つのモードを実装する
# 0: current git status & current server
# 1: arg branch & current server
# 2: arg branch & multi server

GIT_TOP_PATH=$( git rev-parse --show-toplevel )

APP_PROFILE_DIR=/home/isucon/profile

DATE=$( date --iso-8601=seconds )

source ./env.sh

for server in ${APP_SERVERS[@]}
do
    # Update git                                                                                                                                                                                                                                 # git checkout master
    # git stash
    # git pull origin master
    # git stash apply stash@{0}

    # Rotate log files
    for LOG_FILE in "${LOG_FILES[@]}"; do
        ssh isucon@${server} "sudo test -f ${LOG_FILE} && sudo mv ${LOG_FILE} ${LOG_FILE}.${DATE} || echo 1"
    done

    mkdir -p ${APP_PROFILE_DIR}

    # locate file
    for common_path in ${COMMON_PATHS[@]}
    do
        rsync -auvz -e ssh --rsync-path='sudo rsync' $( basename ${common_path} ) isucon@${server}:${common_path}
    done
    for uncommon_path in ${UNCOMMON_PATHS[@]}
    do
        rsync -auvz -e ssh --rsync-path='sudo rsync' $( basename ${uncommon_path} ).${server} isucon@${server}:${uncommon_path}
    done

    # build app
    ssh isucon@${server} "cd ${APP_DIR} && ~/local/go/bin/go build"
    
    # Restart services
    ssh isucon@${server} sudo systemctl restart "${SERVICES[@]}"

done
