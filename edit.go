package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/jcs/id3-go"
	"github.com/jcs/id3-go/v2"
	log "github.com/sirupsen/logrus"
)

// EditAlbum is an editable representation of the tags I care about
type EditAlbum struct {
	Album  string
	Artist string
	Genre  string
	Year   string
	Tracks []string
}

// EditData is the root stucture that will be serialized for editing
type EditData struct {
	Albums []EditAlbum
}

// StripTrailingNull cleans up after id3-go's mess
func StripTrailingNull(s string) string {
	if s == "" {
		return s
	}
	return s[0 : len(s)-1]
}

func ParseEditAlbum(a Album) EditAlbum {
	log.WithField("file", a.OrigFile).Print("Parsing tags for album info")
	first, err := id3.Open(a.Tracks[0])
	defer first.Close()
	if err != nil {
		log.WithField("file", a.OrigFile).WithError(err).Fatal("Error parsing tags")
	}

	tracks := []string{}
	for _, track := range a.Tracks {
		log.WithField("file", track).Debug("\tParsing tags for track info")
		tag, err := id3.Open(track)
		if err != nil {
			log.WithField("file", track).WithError(err).Fatal("Error parsing tags")
		}
		//assuming there won't be many mp3s in a zip
		defer tag.Close()
		tracks = append(tracks, StripTrailingNull(tag.Title()))
	}

	return EditAlbum{
		Artist: StripTrailingNull(first.Artist()),
		Album:  StripTrailingNull(first.Album()),
		Genre:  StripTrailingNull(first.Genre()),
		Year:   StripTrailingNull(first.Year()),
		Tracks: tracks,
	}
}

func ParseEditData(albums []Album) EditData {
	edits := []EditAlbum{}
	for _, album := range albums {
		edit := ParseEditAlbum(album)
		edits = append(edits, edit)
	}
	return EditData{
		Albums: edits,
	}
}

func ApplyEditToTrack(origFile string, edit EditAlbum, track string, trackNum int) {
	log.WithField("track", track).Debug("Updating tags")
	tags, err := id3.Open(track)
	if err != nil {
		log.WithField("track", track).WithError(err).Fatal("Error updating track")
	}
	defer tags.Close()

	tags.SetArtist(edit.Artist)
	tags.SetAlbum(edit.Album)
	tags.SetGenre(edit.Genre)
	tags.SetYear(edit.Year)
	tags.SetTitle(edit.Tracks[trackNum])
	// set album artist
	tags.DeleteFrames("TPE2")
	tags.AddFrames(v2.NewTextFrame(v2.V23FrameTypeMap["TPE2"], edit.Artist))
	//add comments
	comments := fmt.Sprintf("Imported by bci from %s on %s", origFile, time.Now())
	tags.AddFrames(v2.NewUnsynchTextFrame(v2.V23FrameTypeMap["COMM"], "bci", comments))
}

func ApplyEditData(edit EditData, albums []Album) {
	log.Print("Updating tags")
	for i, editAlbum := range edit.Albums {
		album := &albums[i]
		for i, track := range album.Tracks {
			ApplyEditToTrack(album.OrigFile, editAlbum, track, i)
		}
	}
}

func WriteEdit(file string, edit EditData) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	err = enc.Encode(edit)
	return err
}

func ReadEdit(file string, edit *EditData) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = toml.DecodeReader(f, edit)
	return err
}

func Edit(work string, albums []Album) EditData {
	edit := ParseEditData(albums)

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	editor, err := exec.LookPath(editor)
	if err != nil {
		log.WithField("editor", editor).WithError(err).Fatal("Error locating editor")
	}

	editFile := filepath.Join(work, "edit.toml")
	log.WithField("file", editFile).Print("Writing temp file")
	err = WriteEdit(editFile, edit)
	if err != nil {
		log.WithField("file", editFile).WithError(err).Fatal("Error writing temp file")
	}

	log.WithField("editor", editor).Print("Invoking editor")
	cmd := exec.Command(editor, editFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.WithError(err).Fatal("Error when invoking editor")
	}

	log.WithField("file", editFile).Print("Reading temp file")
	err = ReadEdit(editFile, &edit)
	if err != nil {
		log.WithField("file", editFile).WithError(err).Fatal("Error reading temp file")
	}

	ApplyEditData(edit, albums)
	return edit
}
