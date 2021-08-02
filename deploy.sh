#!/usr/bin/env bash
set -eux

# 3つのモードを実装する
# 0: current git status & current server
# 1: arg branch & current server
# 2: arg branch & multi server

GIT_TOP_PATH=$( git rev-parse --show-toplevel )

SERVERS=(
    $( hostname )
)

LOG_FILES=(
    /var/log/mysql/mysql-slow.log
    /var/log/mysql/access.log
    /var/log/mysql/error.log
)

SERVICES=(
    nginx
    mysql
    isucari.python
)

DATE=$( date --iso-8601=seconds )

source ./deployed_file_paths.sh

for server in ${SERVERS[@]}
do
    # Update git                                                                                                                                                                                                                                 # git checkout master
    # git stash
    # git pull origin master
    # git stash apply stash@{0}

    # Rotate log files
    for LOG_FILE in "${LOG_FILES[@]}"; do
        ssh isucon@${server} "sudo test -f ${LOG_FILE} && sudo mv ${LOG_FILE} ${LOG_FILE}.${DATE} || echo 1"
    done

    # locate file
    for common_path in ${COMMON_PATHS[@]}
    do
        rsync -auvz -e ssh --rsync-path='sudo rsync' $( basename ${common_path} ) isucon@${server}:${common_path}
    done
    for uncommon_path in ${UNCOMMON_PATHS[@]}
    do
        rsync -auvz -e ssh --rsync-path='sudo rsync' $( basename ${uncommon_path} ).${server} isucon@${server}:${uncommon_path}
    done

    # Restart services
    ssh isucon@${server} sudo systemctl restart "${SERVICES[@]}"
done
