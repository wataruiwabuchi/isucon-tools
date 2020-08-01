#!/usr/bin/env bash
set -ex
export PATH=$HOME/local/go/bin:$PATH

GITHOME=`cd $(dirname "${0}") && cd .. && pwd`

LOG_FILES=(
    /var/log/mysql/mysql-slow.log
    /var/log/mysql/error.log
)

SERVICES=(
    nginx
    mysql
    isubata.golang
)

DATE=`date "+%Y%m%d_%H%M%S"`

# Move to working directory
cd "${GITHOME}"

# Update git
# git checkout master
# git stash
# git pull origin master
# git stash apply stash@{0}

# Rotate log files
for LOG_FILE in "${LOG_FILES[@]}"; do
    sudo test -f "${LOG_FILE}" && sudo mv "${LOG_FILE}" "${LOG_FILE}.${DATE}"
done

cd "${HOME}/isubata/webapp/go"
make

# Restart services
sudo systemctl restart "${SERVICES[@]}"
