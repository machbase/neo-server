DIR_DEVELOPER=$HOME/Developer
GIT_REPO=dbms-mach-go
GIT_ORG=MACHBASE

GIT_URL=git@github.com:$GIT_ORG/$GIT_REPO.git

export PATH=$PATH:/$HOME/go/bin
export GOPRIVATE=github.com/machbase/*

if [ ! -d $DIR_DEVELOPER ]; then
    mkdir -p  $DIR_DEVELOPER 
fi

cd $DIR_DEVELOPER

if [ ! -d $GIT_REPO ]; then
    git clone $GIT_URL && cd $GIT_REPO
else
    cd $GIT_REPO && git pull
fi

make package-machbase-neo && mv packages/machbase-neo-v*.zip /tmp
