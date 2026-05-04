go mod init github.com/aprice2704/appy
git init
git branch -m main
cls
cp -r ../fdm/cmd/app/* .
cp -r ../fdm/code/cmd/app/* .
cp -r ../fdm/code/cmd/appy/* .
tre
git remote add origin git@github.com:aprice2704/appy.git
git push -u origin main
git remote add origin https://github.com/aprice2704/appy.git
git remote delete origin
git remote remove origin
git push -u origin main
cls
git status
git add *
git push -u origin main
git commit -m "initial v1.5.15"
git push -u origin main
git remote remove origin
git remote add origin git@github.com:aprice2704/appy.git
git push -u origin main
git rm --cached .bash_history
ll -a
cls
git rm --cached .bash_history
echo ".bash_history" >> .gitignore
git restore --staged .bash_history
git status
git add .gitignore
go mod edit -replace github.com/aprice2704/fdm/code/patcheng=../fdm/code/patcheng
go mod tidy
go mod edit -replace github.com/aprice2704/fdm=../fdm
go mod tidy
go mod edit -replace github.com/aprice2704/neuroscript=../neuroscript
go mod tidy
cls
go mod tidy
cp ../fdm/run_appy.sh 
cp ../fdm/run_appy.sh .
cp ../fdm/code/how_it_works/always/agent_patching.md .
licecomb
licecomb -rewrite
cls
ll -a
tre -a
cls
./run_appy.sh 
chmod +x scripts/playground.sh && ./scripts/playground.sh
t
./scripts/playground.sh
./scripts/playground.sh -port 8087
go install .
t
echo -e 'package main\n\nfunc Helper() {\n\tprintln("help")\n}' > playground/helper.go
t
go install .
./scripts/playground.sh -port 8087
go get github.com/chromedp/chromedp
t
go mod download
go mod tidy
t
go mod tidy
GOPROXY=https://proxy.golang.org,direct go get github.com/chromedp/cdp@latest
go mod tidy
cls
GOPROXY=https://proxy.golang.org,direct go get github.com/chromedp/cdp@latest
go clean -modcache
env GOPRIVATE="" GONOPROXY="" GOPROXY=https://proxy.golang.org GIT_TERMINAL_PROMPT=0 go get github.com/chromedp/chromedp@latest github.com/chromedp/cdp@latest
t
tre
x
go install .
mkdir -p testdata
mv scripts/fixtures testdata/
rm -rf playground
t
tre
./run_appy.sh -port 8086
cat *.go
git status
git add *
du -h --max-depth=2 . | sort -h | tail -40
git count-objects -vH
git status --short | wc -l
git status --short | head -50
echo '/appy_bin' >> .gitignore
git rm --cached appy_bin
git gc
cls
git gc
git status --short | head -50
du -h --max-depth=2 . | sort -h | tail -40
cls
ps -A 
cls
ps -A 
sudo iotop -oPa
sudo apt install iotop
sudo iotop -oPa
balooctl6 suspend 2>/dev/null || balooctl suspend
balooctl6 disable
balooctl6 purge
git gc
balooctl6 status 2>/dev/null || balooctl status
cls
systemctl --user disable --now kde-baloo.service 2>/dev/null
systemctl --user mask kde-baloo.service 2>/dev/null
t
licecomb
licecomb -rewrite
licecomb
cls
txtar * > appy.txtar
txtar ** > appy.txtar
txtar **/** > appy.txtar
txtar *.go *.js  > appy.txtar
txtar ./*.go ./*.js  > appy.txtar
txtar -h
rm appy_bin 
cat run_appy.sh 
cls
tre
cat ./scripts/playground.sh 
cls
rm appy.txtar 
cd scripts/
txtar -h
txtar c "**/*.go" "**/*.js" > context.txtar
txtar c "**/*.go" **/*.js > context.txtar
txtar c **/*.go **/*.js **/*.md  > appy.txtar
rm context.txtar 
txtar l appy.txtar 
cls
txtar c **/*.go **/*.js **/*.md  > appy.txtar
./run_appy.sh 
go install .
t
go install .
which go
go version
go env GOROOT GOPATH GOTOOLCHAIN GOWORK GOMOD
t
ll -a
t
go install .
t
go install .
t
