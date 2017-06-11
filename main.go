package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	log "github.com/sirupsen/logrus"
)

type Album struct {
	OrigFile string
	Dir      string
	Tracks   []string
}

func ExtractAlbum(work string, file string) Album {
	base := filepath.Base(file)
	tempPath := filepath.Join(work, base)
	fields := log.Fields{
		"zip": file,
		"dir": tempPath,
	}
	log.WithFields(fields).Print("Extracting")
	err := Unzip(file, tempPath)
	if err != nil {
		log.WithFields(fields).WithError(err).Fatal("Error unzipping")
	}

	tracks, err := filepath.Glob(filepath.Join(tempPath, "*.mp3"))
	if err != nil {
		log.WithFields(fields).WithError(err).Fatal("Error finding extracted mp3s")
	}
	sort.Strings(tracks)

	return Album{
		OrigFile: base,
		Dir:      tempPath,
		Tracks:   tracks,
	}
}

func ExtractAlbums(work string, files []string) []Album {
	albums := []Album{}
	for _, file := range files {
		album := ExtractAlbum(work, file)
		albums = append(albums, album)
	}
	return albums
}

func Copy(src, dst string) {
	dstFile, err := os.Create(dst)
	if err != nil {
		log.WithField("file", dst).WithError(err).Fatal("Error creating file for copying")
	}
	srcFile, err := os.Open(src)
	if err != nil {
		log.WithField("file", src).WithError(err).Fatal("Error opening file for copying")
	}
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		log.WithField("file", dst).WithError(err).Fatal("Error copying file")
	}
}

func CopyAlbum(root string, album Album, edit EditAlbum) {
	dir := filepath.Join(root, edit.Artist, edit.Album)
	log.WithField("dir", dir).Print("Creating destination directory")
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		log.WithField("dir", dir).WithError(err).Fatal("Error creating directory")
	}
	for i, track := range album.Tracks {
		filename := fmt.Sprintf("%02d - %s.mp3", i+1, edit.Tracks[i])
		dst := filepath.Join(dir, filename)
		Copy(track, dst)
	}
}

func CopyAlbums(root string, albums []Album, edit EditData) {
	for i, album := range albums {
		CopyAlbum(root, album, edit.Albums[i])
	}
}

func main() {
	root := flag.String("root", ".", "The root import directory, will be created if needed")
	work := flag.String("work", filepath.Join(os.TempDir(), "bci"), "The working directory, will be created if needed, and delete after completion.")
	flag.Parse()
	if *root == "" || *work == "" || len(flag.Args()) == 0 {
		fmt.Println("usage: bci [flags] zips+")
		flag.PrintDefaults()
		return
	}
	albums := ExtractAlbums(*work, flag.Args())
	edit := Edit(*work, albums)
	CopyAlbums(*root, albums, edit)
	os.RemoveAll(*work)
}
