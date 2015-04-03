FROM golang:1-onbuild

RUN apt-get update && apt-get install vim-nox tmux -y

ENTRYPOINT ["/go/bin/app", "0.0.0.0:1234"]
CMD ["/bin/bash"]
