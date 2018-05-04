# k8s-info

Command line utility to get basic metrics and pod breakdown of a kubernetes cluster. Similar to `kubectl top {resource}`

## Install and run
1. Install Go [https://golang.org/dl/](https://golang.org/dl/)
2. Clone repository
3. If using vscode press F5 to run alternatively run `go run main.go metrics.go`

N.B If vendor folder gets deleted running `godep restore ./...` will get all required packages

## Parameters
* kubeconfig = Specify absolute path to kubeconfig file (Optional)
* namespace  = Specify namespace to get resource from (Optional) (--namespace test OR -namespace=test)
* watch      = Watch cluster at 15 sec interval (Optional) (--watch OR -watch)
