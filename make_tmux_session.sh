#!/bin/bash

set -ue

SESSION_NAME="isucon"
SENDED_COMMANDS=(
  #"ssh image1"
  "cd /home/isucon/isubata/webapp/go"
)
WINDOW_NAMES=("glances" "nginx" "mysql" "app")

tmux new-session -d  -s $SESSION_NAME

for WINDOW_NAME in "${WINDOW_NAMES[@]}"
do
  tmux new-window -n $WINDOW_NAME

  for COMMAND in "${SENDED_COMMANDS[@]}"
  do  
    tmux send-keys "$COMMAND" C-m
  done
done

tmux attach -t $SESSION_NAME
