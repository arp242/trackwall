all: build

build:
	./build.sh

install: build
	./install.sh

clean:
	rm -vf trackwall
	rm -rvf pkg
