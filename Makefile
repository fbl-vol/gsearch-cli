.PHONY: install-git-hooks validate

install-git-hooks:
	bash scripts/install-git-hooks.sh

# The baseline is local and credential-free; live API checks remain opt-in.
validate:
	go test ./...
	go vet ./...
	go build ./...
