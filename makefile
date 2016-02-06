export GOPATH=$(CURDIR)/.go

APP_NAME = recause
OUTDIR=$(CURDIR)/out
DEBIAN_TMP=$(OUTDIR)/deb
VERSION=`$(OUTDIR)/$(APP_NAME) -v | cut -d ' ' -f 3`

$(OUTDIR)/$(APP_NAME): $(CURDIR)/src/main.go
	go build -o $(OUTDIR)/$(APP_NAME) $(CURDIR)/src/main.go
	chmod 0755 $(OUTDIR)/$(APP_NAME)

deb: $(OUTDIR)/$(APP_NAME)
	mkdir $(DEBIAN_TMP)
	mkdir -p $(DEBIAN_TMP)/etc/$(APP_NAME)
	mkdir -p $(DEBIAN_TMP)/usr/local/bin
	install -m 644 $(CURDIR)/data/sample.cfg $(DEBIAN_TMP)/etc/$(APP_NAME)/config.cfg
	install -m 755 $(OUTDIR)/$(APP_NAME) $(DEBIAN_TMP)/usr/local/bin
	fpm -n $(APP_NAME) \
		-v $(VERSION) \
		-t deb \
		-s dir \
		-C $(DEBIAN_TMP) \
		-p $(OUTDIR) \
		--config-files   /etc/$(APP_NAME) \
		--after-install  $(CURDIR)/debian/postinst \
		--after-remove   $(CURDIR)/debian/postrm \
		--deb-init       $(CURDIR)/debian/$(APP_NAME) \
		.
	rm -rf $(DEBIAN_TMP)

dep-install:
	go get github.com/braintree/manners
	go get github.com/codegangsta/cli
	go get github.com/endeveit/go-gelf/gelf
	go get github.com/endeveit/go-snippets/...
	go get github.com/gorilla/mux
	go get github.com/satori/go.uuid
	go get github.com/Sirupsen/logrus
	go get gopkg.in/olivere/elastic.v3

dep-update:
	go get -u github.com/braintree/manners
	go get -u github.com/codegangsta/cli
	go get -u github.com/endeveit/go-gelf/gelf
	go get -u github.com/endeveit/go-snippets/...
	go get -u github.com/gorilla/mux
	go get -u github.com/satori/go.uuid
	go get -u github.com/Sirupsen/logrus
	go get -u gopkg.in/olivere/elastic.v3

fmt:
	gofmt -s=true -w $(CURDIR)/src

run:
	go run -v $(CURDIR)/src/main.go -c=$(CURDIR)/data/sample.cfg -p=$(CURDIR)/recause.pid

strip: $(OUTDIR)/$(APP_NAME)
	strip $(OUTDIR)/$(APP_NAME)

clean:
	rm -f $(OUTDIR)/*

clean-deb:
	rm -rf $(DEBIAN_TMP)
	rm -f $(OUTDIR)/*.deb
