set fallback

default:
    just --choose

list:
    just --list


import? 'justfile.custom'

CGO_ENABLED := env_var_or_default('CGO_ENABLED', '0')
GO111MODULE := env_var_or_default('GO111MODULE', 'on')
GOFLAGS := env_var_or_default('GOFLAGS', '"-mod=mod"')
SUFFIX := env_var_or_default('SUFFIX', '')

download:
    @echo Download go.mod dependencies
    go mod download

prepush: vet fix fmt tidy test

test:
    CGO_ENABLED=0 go build ./...
    CGO_ENABLED=1 go test -race -cover -v -mod=readonly ./... && echo -e "\033[32mSUCCESS\033[0m" || (echo -e "\033[31mFAILED\033[0m" && exit 1)

bench:
    go test -test.timeout=30m -benchmem -run ^$$ -benchtime=20s -bench . ./... && echo -e "\033[32mSUCCESS\033[0m" || (echo -e "\033[31mFAILED\033[0m" && exit 1)

vet:
    go vet ./...

fix:
    go fix ./...

fmt:
    go fmt ./...

tidy:
    go mod tidy

verify:
    go mod verify

go_list:
    go list -u -m all

go_update_all:
    go get -t -u ./...

