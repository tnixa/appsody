#!/bin/bash
set -e
mkdir ./tmpdocclone
cd ./tmpdocclone
git clone https://${GH_TOKEN}@github.com/${GH_ORG}/docs.git

cd docs

set +e
diff ../../build/cli-commands.md ./docs/using-appsody/cli-commands.md
if [ $? -ne 0 ]
then
    set -e
    git checkout -b test${TRAVIS_BUILD_NUMBER}
    cp ../../build/cli-commands.md ./docs/using-appsody/cli-commands.md

    git add docs/using-appsody/cli-commands.md

    git commit -m "Travis build: $TRAVIS_BUILD_NUMBER" --author="Kyle G. Christianson <christik@us.ibm.com>"

    git push --set-upstream origin test${TRAVIS_BUILD_NUMBER}

fi
# clean up
cd ../..
rm -rf tmpdocclone

 
