GO_LINKER_FLAGS=-ldflags="-s -w"

APP_NAME=recause
MAIN_GO=$(CURDIR)/main.go
GO_PROJECT_FILES=`go list -f '{{.Dir}}' ./... | grep -v /vendor/ | grep -v '$(APP_NAME)$$'`
GO_PROJECT_FILES+=$(MAIN_GO)

# Useful directories
DIR_BUILD=$(CURDIR)/_build
DIR_OUT=$(DIR_BUILD)/out
DIR_OUT_LINUX=$(DIR_OUT)/linux
DIR_DEBIAN_TMP=$(DIR_OUT)/deb
DIR_RESOURCES=$(DIR_BUILD)/resources

# Remove the "v" prefix from version
VERSION=`$(DIR_OUT_LINUX)/$(APP_NAME) -v | cut -d ' ' -f 3 | tr -d 'v'`

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

.build-linux:
	@echo Build Linux amd64
	@env GOOS=linux GOARCH=amd64 go build -o $(DIR_OUT_LINUX)/$(APP_NAME) $(GO_LINKER_FLAGS) $(MAIN_GO)

.build-osx:
	@echo Build OSX amd64
	@env GOOS=darwin GOARCH=amd64 go build -o $(DIR_OUT)/darwin/$(APP_NAME) $(GO_LINKER_FLAGS) $(MAIN_GO)

# Default make target
build: check .build-linux .build-osx

# Launch all checks
check: .vet .errcheck

# Build deb-package with Effing Package Management (https://github.com/jordansissel/fpm)
deb: check .build-linux
	@echo Build debian package
	@mkdir $(DIR_DEBIAN_TMP)
	@mkdir -p $(DIR_DEBIAN_TMP)/etc/$(APP_NAME)
	@mkdir -p $(DIR_DEBIAN_TMP)/usr/local/bin
	@install -m 644 $(DIR_RESOURCES)/sample.cfg $(DIR_DEBIAN_TMP)/etc/$(APP_NAME)/config.cfg
	@install -m 755 $(DIR_OUT_LINUX)/$(APP_NAME) $(DIR_DEBIAN_TMP)/usr/local/bin
	fpm -n $(APP_NAME) \
		-v $(VERSION) \
		-t deb \
		-s dir \
		-C $(DIR_DEBIAN_TMP) \
		-p $(DIR_OUT) \
		--config-files   /etc/$(APP_NAME) \
		--after-install  $(CURDIR)/build/debian/postinst \
		--after-remove   $(CURDIR)/build/debian/postrm \
		--deb-init       $(CURDIR)/build/debian/$(APP_NAME) \
		.
	@rm -rf $(DIR_DEBIAN_TMP)

# Format the source code
fmt:
	@gofmt -s=true -w $(GO_PROJECT_FILES)

# Run the program from CLI without compilation for testing purposes
run:
	go run -v $(MAIN_GO) -c=$(DIR_RESOURCES)/sample.cfg -p=$(DIR_OUT)/recause.pid

# Bootstrap vendoring tool and dependencies
bootstrap:
	@for tool in  $(EXTERNAL_TOOLS) ; do \
		echo "Installing $$tool" ; \
		go get -u $$tool; \
	done
	#@echo "Installing dependencies"; glide install

# Launch tests
test:
	@go test `go list ./... | grep -v /vendor/ | grep -v '$(APP_NAME)$$'`
