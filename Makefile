
all: followthestock

followthestock: *.go
	go get -v ; go build -v -o followthestock

test: followthestock
	go test -v

install:
	# Config
	mkdir -p $(DESTDIR)/etc/followthestock
	cp followthestock.conf $(DESTDIR)/etc/followthestock/followthestock.conf

	# Binary
	mkdir -p $(DESTDIR)/usr/bin
	cp followthestock $(DESTDIR)/usr/bin/followthestock

	# Data dir
	mkdir -p $(DESTDIR)/var/lib/followthestock

	# Logging
	mkdir -p $(DESTDIR)/var/log/followthestock $(DESTDIR)/etc/logrotate.d
	cp package/logrotate.d/* $(DESTDIR)/etc/logrotate.d

	# Startup scripts
	mkdir -p $(DESTDIR)/etc/supervisor/conf.d
	cp package/supervisor.d/*.conf $(DESTDIR)/etc/supervisor/conf.d

package: followthestock
	dpkg-buildpackage -b -us -uc
	mkdir -p dist/package
	mv ../*.deb dist/package/
	rm ../*.changes

test_package_local:
	make clean
	make package
	sudo dpkg -i dist/package/*.deb

clean:
	rm -Rf dist followthestock


test_package_remote:
	make package
	rsync -avz --progress dist/package/*.deb $(TARGET):followthestock.deb
	ssh $(TARGET) dpkg -i followthestock.deb

run: followthestock
	./followthestock
