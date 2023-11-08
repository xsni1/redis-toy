package main

import (
	"log"
	"syscall"
)

func main() {
	// O_NONBLOCK - ustawia fd socketa na nonblocking
	// wowczas w momencie gdy jakas operacja jest blokujaca zwracany jest blad EAGAIN
	// fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM|syscall.SOCK_NONBLOCK, 0)

	if err != nil {
		log.Fatalf("err during syscall socket: %s", err)
	}
	defer syscall.Close(fd)

	addr := syscall.SockaddrInet4{Port: 6378}
	err = syscall.Bind(fd, &addr)
	if err != nil {
		log.Fatalf("err during syscall bind: %s", err)
	}

	err = syscall.Listen(fd, 10)
	if err != nil {
		log.Fatalf("err during syscall listen: %s", err)
	}

	log.Printf("syscall socket fd: %d, address: %+v", fd, addr)

	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		log.Fatalf("err during epoll create: %s", err)
	}

	// epoll control call, dodajemy fd jaki epoll powinien pollowac - tutaj "glowny" fd serwujacy
	err = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, fd, &syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(fd)})
	if err != nil {
		log.Fatalf("err during epoll control: %s", err)
	}

	var epollEvents [32]syscall.EpollEvent
	for {
		nevents, err := syscall.EpollWait(epfd, epollEvents[:], 30*1000)
		if err != nil {
			log.Fatalf("err during epollwait: %s", err)
		}

		for i := 0; i < nevents; i++ {
			e := epollEvents[i]
			log.Printf("%+v", e)

			// jesli fd serwera zostal aktywny - nowe polaczenie/dc
			if int(e.Fd) == fd {
				// if e.Events & syscall.EPOLLRDHUP {}
				nfd, _, err := syscall.Accept(fd)
				if err != nil {
					log.Printf("err during new socket accept: %s", err)
				}
				syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, nfd, &syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(nfd)})
				log.Printf("added new socket %d", nfd)
			} else {
				buf := make([]byte, 1024)
				// co jak read nie przeczyta calosci?
				nread, err := syscall.Read(int(e.Fd), buf)
				if err != nil {
					log.Printf("err during read: %s", err)
				}
				// if eof - close tcp connection
				if nread == 0 {
					log.Printf("read 0, disconnecting %d", e.Fd)
					syscall.Close(int(e.Fd))
					continue
				}
				log.Printf("%d, %s", e.Fd, string(buf))
				syscall.Write(int(e.Fd), []byte("PONG\n"))
			}
		}
	}
}
