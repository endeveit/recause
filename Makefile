GO_LINKER_FLAGS=-ldflags="-s -w"

APP_NAME=recause
SRC_RECAUSE=github.com/endeveit/recause/cmd/recause
GO_PROJECT_FILES=`go list -f '{{.Dir}}' ./... | grep -v /vendor/ | grep -v '$(APP_NAME)$$'`

# Useful directories
DIR_BUILD=$(CURDIR)/_build
DIR_OUT=$(DIR_BUILD)/out
DIR_OUT_LINUX=$(DIR_OUT)/linux
DIR_DEBIAN_TMP=$(DIR_OUT)/deb
DIR_RESOURCES=$(DIR_BUILD)/resources

EXTERNAL_TOOLS=\
	github.com/kisielk/errcheck \
	github.com/Masterminds/glide

# Check for suspicious constructs
.vet:
	@for project_file in $(GO_PROJECT_FILES); do \
		go tool vet $$project_file; \
		if [ $$? -eq 1 ]; then \
			echo ""; \
			echo "Vet found suspicious constructs. Please check the reported constructs"; \
			echo "and fix them if necessary."; \
		fi \
	done

# Check the go files for unchecked errors
.errcheck:
	@for project_file in $(GO_PROJECT_FILES); do \
		if [ -f $$project_file ]; then \
			errcheck $$project_file; \
		else \
			errcheck $$(find $$project_file -type f); \
		fi \
	done

# Default make target
build-all: build-linux-amd64 build-linux-arm build-osx

build-linux-amd64:
	@echo Build Linux amd64
	@env GOOS=linux GOARCH=amd64 go build -o $(DIR_OUT_LINUX)/amd64/$(APP_NAME) $(GO_LINKER_FLAGS) $(SRC_RECAUSE)

build-linux-arm:
	@echo Build Linux armhf
	@env GOOS=linux GOARCH=arm go build -o $(DIR_OUT_LINUX)/armhf/$(APP_NAME) $(GO_LINKER_FLAGS) $(SRC_RECAUSE)

build-osx:
	@echo Build OSX amd64
	@env GOOS=darwin GOARCH=amd64 go build -o $(DIR_OUT)/darwin/$(APP_NAME) $(GO_LINKER_FLAGS) $(SRC_RECAUSE)

# Launch all checks
check: .vet .errcheck

# Build deb-package with Effing Package Management (https://github.com/jordansissel/fpm)
deb: build-linux-amd64 build-linux-arm
	@echo Build debian packages
	@rm -f $(DIR_OUT)/*.deb
	@mkdir -p $(DIR_DEBIAN_TMP)
	@mkdir -p $(DIR_DEBIAN_TMP)/etc/$(APP_NAME)
	@mkdir -p $(DIR_DEBIAN_TMP)/usr/local/bin
	@install -m 644 $(DIR_RESOURCES)/sample.cfg $(DIR_DEBIAN_TMP)/etc/$(APP_NAME)/config.cfg
	$(eval ARCH = $(shell go env GOARCH))
	$(eval VERSION=`$(DIR_OUT_LINUX)/$(shell go env GOARCH)/$(APP_NAME) -v | cut -d ' ' -f 3 | tr -d 'v'`)

	@for arch in "amd64" "armhf" ; do \
		install -m 755 $(DIR_OUT_LINUX)/$$arch/$(APP_NAME) $(DIR_DEBIAN_TMP)/usr/local/bin; \
		fpm -s dir \
			-C $(DIR_DEBIAN_TMP) \
			-p $(DIR_OUT) \
			--after-install $(DIR_BUILD)/debian/postinst \
			--after-remove $(DIR_BUILD)/debian/postrm \
			--deb-init $(DIR_BUILD)/debian/$(APP_NAME) \
			-a $$arch \
			-t deb \
			-n $(APP_NAME) \
			-v $(VERSION) \
			--log error \
			.; \
	done

	@rm -rf $(DIR_DEBIAN_TMP)

# Format the source code
fmt:
	@gofmt -s=true -w $(GO_PROJECT_FILES)

# Run the program from CLI without compilation for testing purposes
run:
	go run -v $(SRC_RECAUSE) -c=$(DIR_RESOURCES)/sample.cfg -p=$(DIR_OUT)/recause.pid

# Bootstrap vendoring tool and dependencies
bootstrap:
	@for tool in $(EXTERNAL_TOOLS) ; do \
		echo "Installing $$tool" ; \
		go get -u $$tool; \
	done
	@echo "Installing dependencies"; glide install

# Launch tests
test:
	@go test `go list ./... | grep -v /vendor/ | grep -v '$(APP_NAME)$$'`
