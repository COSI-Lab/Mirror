#!/bin/bash

/usr/bin/tmux new-session -d -s mirror "cd /home/mirror/Mirror && git pull && go build && ./Mirror"