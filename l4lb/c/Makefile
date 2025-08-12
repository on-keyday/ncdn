# configurations
NCDN_STRIP=0

CFLAGS=-O2 -g -Wall -Wno-unused-value -Wno-pointer-sign
CFLAGS+=-Wno-compare-distinct-pointer-types -Wno-address-of-packed-member -Werror -fno-builtin-bcmp

all: lb.o dummy.o init_crypto.o

.PHONY: all clean

.SUFFIXES:
.SUFFIXES: .o .ll .c .h

.c.ll:
	clang-19 ${CFLAGS} -emit-llvm -c -o $@ $<

.ll.o:
	llc-19 -march=bpf -filetype=obj -o $@ $<
ifeq (${NCDN_STRIP},1)
	llvm-strip -g $@
endif

%.o: %.c

clean:
	rm -f *.ll *.o
