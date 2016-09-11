package main

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type result struct {
	path string
	sum  [md5.Size]byte
	err  error
}

func main() {
	now := time.Now()
	m, err := MD5ALL(os.Args[1])
	if err != nil {
		fmt.Println(err)
		return
	}

	var paths []string
	for path := range m {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		fmt.Printf("%x %s\n", m[path], path)
	}
	elapsed := time.Since(now)
	fmt.Println(elapsed)
}

func MD5ALL(root string) (map[string][md5.Size]byte, error) {
	done := make(chan struct{})
	defer close(done)

	paths, errc := walkFiles(done, root)

	c := make(chan result)
	var wg sync.WaitGroup
	const numDigesters = 20
	wg.Add(numDigesters)
	for i := 0; i < numDigesters; i++ {
		go func() {
			digester(done, paths, c)
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(c)
	}()

	m := make(map[string][md5.Size]byte)
	for r := range c {
		if r.err != nil {
			return nil, r.err
		}
		m[r.path] = r.sum
	}

	if err := <-errc; err != nil {
		return nil, err
	}
	return m, nil
}

func walkFiles(done <-chan struct{}, root string) (<-chan string, <-chan error) {
	paths := make(chan string)
	errc := make(chan error, 1)

	go func() {
		defer close(paths)
		errc <- filepath.Walk(root,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.Mode().IsRegular() {
					return nil
				}
				select {
				case paths <- path:
				case <-done:
				}
				return nil
			})
	}()
	return paths, errc
}

func digester(done <-chan struct{}, paths <-chan string, c chan<- result) {
	for path := range paths {
		data, err := ioutil.ReadFile(path)
		select {
		case c <- result{path, md5.Sum(data), err}:
		case <-done:
			return
		}
	}
}
