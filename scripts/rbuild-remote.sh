DIR_BASE=$HOME/Developer/neo
GIT_REPOS=("neo-engine" "neo-grpc" "neo-server")
GIT_ORG=MACHBASE

export PATH=$PATH:/$HOME/go/bin
export GOPRIVATE=github.com/machbase/*

if [ ! -d $DIR_BASE ]; then
    mkdir -p  $DIR_BASE 
fi

for REPO in ${GIT_REPOS[@]}; do
    cd $DIR_BASE
    if [ ! -d $REPO ]; then
        git clone git@github.com:$GIT_ORG/$REPO.git && cd $REPO
    else
        cd $REPO && git pull
    fi
done

cd $DIR_BASE/neo-server

make package-machbase-neo && mv packages/machbase-neo-v*.zip /tmp
