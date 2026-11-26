package study

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"database/sql"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	ankiSchemaVersion = 11
	ankiBasicModelID  = int64(1607392319)
	ankiDeckIDBase    = int64(1700000000)
	ankiFieldSep      = "\x1f"
)

func BuildAnkiAPKG(deckName string, cards []StudyCard) ([]byte, error) {
	deckName = strings.TrimSpace(deckName)
	if deckName == "" {
		deckName = "Shelfd Study Deck"
	}

	tmpFile, err := os.CreateTemp("", "shelfd-anki-*.anki2")
	if err != nil {
		return nil, fmt.Errorf("create temp sqlite file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpPath)

	db, err := sql.Open("sqlite3", tmpPath)
	if err != nil {
		return nil, fmt.Errorf("open temp sqlite db: %w", err)
	}

	if err := populateAnkiDB(db, deckName, cards); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := db.Close(); err != nil {
		return nil, fmt.Errorf("close temp sqlite db: %w", err)
	}

	dbBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read temp sqlite db: %w", err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	colEntry, err := zw.Create("collection.anki2")
	if err != nil {
		return nil, fmt.Errorf("create collection.anki2 zip entry: %w", err)
	}
	if _, err := colEntry.Write(dbBytes); err != nil {
		return nil, fmt.Errorf("write collection.anki2 zip entry: %w", err)
	}

	mediaEntry, err := zw.Create("media")
	if err != nil {
		return nil, fmt.Errorf("create media zip entry: %w", err)
	}
	if _, err := mediaEntry.Write([]byte("{}")); err != nil {
		return nil, fmt.Errorf("write media zip entry: %w", err)
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close apkg zip archive: %w", err)
	}

	return buf.Bytes(), nil
}

func populateAnkiDB(db *sql.DB, deckName string, cards []StudyCard) error {
	now := time.Now().UTC()
	nowSec := now.Unix()
	nowMs := now.UnixMilli()
	deckID := ankiDeckIDBase + (nowSec % 100000)

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin sqlite tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	ddl := []string{
		`CREATE TABLE col (
			id integer primary key,
			crt integer not null,
			mod integer not null,
			scm integer not null,
			ver integer not null,
			dty integer not null,
			usn integer not null,
			ls integer not null,
			conf text not null,
			models text not null,
			decks text not null,
			dconf text not null,
			tags text not null
		)`,
		`CREATE TABLE notes (
			id integer primary key,
			guid text not null,
			mid integer not null,
			mod integer not null,
			usn integer not null,
			tags text not null,
			flds text not null,
			sfld integer not null,
			csum integer not null,
			flags integer not null,
			data text not null
		)`,
		`CREATE TABLE cards (
			id integer primary key,
			nid integer not null,
			did integer not null,
			ord integer not null,
			mod integer not null,
			usn integer not null,
			type integer not null,
			queue integer not null,
			due integer not null,
			ivl integer not null,
			factor integer not null,
			reps integer not null,
			lapses integer not null,
			left integer not null,
			odue integer not null,
			odid integer not null,
			flags integer not null,
			data text not null
		)`,
		`CREATE TABLE revlog (
			id integer primary key,
			cid integer not null,
			usn integer not null,
			ease integer not null,
			ivl integer not null,
			lastIvl integer not null,
			factor integer not null,
			time integer not null,
			type integer not null
		)`,
		`CREATE TABLE graves (
			usn integer not null,
			oid integer not null,
			type integer not null
		)`,
		`CREATE INDEX ix_notes_usn ON notes (usn)`,
		`CREATE INDEX ix_notes_csum ON notes (csum)`,
		`CREATE INDEX ix_cards_usn ON cards (usn)`,
		`CREATE INDEX ix_cards_nid ON cards (nid)`,
		`CREATE INDEX ix_cards_sched ON cards (did, queue, due)`,
		`CREATE INDEX ix_revlog_usn ON revlog (usn)`,
		`CREATE INDEX ix_revlog_cid ON revlog (cid)`,
	}

	for _, stmt := range ddl {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("create anki schema: %w", err)
		}
	}

	confJSON, err := marshalAnkiConf(deckID)
	if err != nil {
		return err
	}
	modelsJSON, err := marshalAnkiModels(deckID, nowSec)
	if err != nil {
		return err
	}
	decksJSON, err := marshalAnkiDecks(deckID, deckName, nowSec)
	if err != nil {
		return err
	}
	dconfJSON, err := marshalAnkiDconf(nowSec)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(
		`INSERT INTO col (id, crt, mod, scm, ver, dty, usn, ls, conf, models, decks, dconf, tags) VALUES (1, ?, ?, ?, ?, 0, 0, 0, ?, ?, ?, ?, '{}')`,
		nowSec, nowMs, nowMs, ankiSchemaVersion, confJSON, modelsJSON, decksJSON, dconfJSON,
	); err != nil {
		return fmt.Errorf("insert anki col row: %w", err)
	}

	noteStmt, err := tx.Prepare(`INSERT INTO notes (id, guid, mid, mod, usn, tags, flds, sfld, csum, flags, data) VALUES (?, ?, ?, ?, -1, ?, ?, ?, ?, 0, '')`)
	if err != nil {
		return fmt.Errorf("prepare note insert: %w", err)
	}
	defer noteStmt.Close()

	cardStmt, err := tx.Prepare(`INSERT INTO cards (id, nid, did, ord, mod, usn, type, queue, due, ivl, factor, reps, lapses, left, odue, odid, flags, data) VALUES (?, ?, ?, 0, ?, -1, 0, 0, ?, 0, 0, 0, 0, 0, 0, 0, 0, '{}')`)
	if err != nil {
		return fmt.Errorf("prepare card insert: %w", err)
	}
	defer cardStmt.Close()

	for i, card := range cards {
		noteID := nowMs + int64(i)
		cardID := noteID + 1000000
		front := strings.TrimSpace(card.Front)
		back := formatCardBackWithAttribution(card)
		fields := front + ankiFieldSep + back
		tags := formatAnkiTags(card.Tags)
		guid := fmt.Sprintf("shelfd-%d-%d", noteID, i)
		csum := ankiFieldChecksum(front)

		if _, err := noteStmt.Exec(noteID, guid, ankiBasicModelID, nowSec, tags, fields, front, csum); err != nil {
			return fmt.Errorf("insert anki note %d: %w", i, err)
		}
		if _, err := cardStmt.Exec(cardID, noteID, deckID, nowSec, i+1); err != nil {
			return fmt.Errorf("insert anki card %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit anki tx: %w", err)
	}
	tx = nil
	return nil
}

func BuildAnkiCSV(cards []StudyCard) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	if err := w.Write([]string{"Front", "Back", "Tags", "BookID", "ChapterIndex", "StartParagraph", "EndParagraph"}); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}

	for _, c := range cards {
		row := []string{
			c.Front,
			c.Back,
			strings.Join(cleanTags(c.Tags), " "),
			c.BookID,
			strconv.Itoa(c.ChapterIndex),
			strconv.Itoa(c.StartParagraph),
			strconv.Itoa(c.EndParagraph),
		}
		if err := w.Write(row); err != nil {
			return nil, fmt.Errorf("write csv row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("flush csv writer: %w", err)
	}
	return buf.Bytes(), nil
}

func formatCardBackWithAttribution(card StudyCard) string {
	back := strings.TrimSpace(card.Back)
	if card.ChapterIndex > 0 || card.StartParagraph > 0 {
		attr := fmt.Sprintf("Chapter %d", card.ChapterIndex)
		if card.StartParagraph > 0 {
			if card.EndParagraph > card.StartParagraph {
				attr += fmt.Sprintf(", ¶%d–%d", card.StartParagraph, card.EndParagraph)
			} else {
				attr += fmt.Sprintf(", ¶%d", card.StartParagraph)
			}
		}
		if back == "" {
			return attr
		}
		return back + "\n\n— " + attr
	}
	return back
}

func cleanTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(strings.ReplaceAll(t, " ", "_"))
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func formatAnkiTags(tags []string) string {
	cleaned := cleanTags(tags)
	if len(cleaned) == 0 {
		return ""
	}
	return " " + strings.Join(cleaned, " ") + " "
}

func ankiFieldChecksum(sfld string) int64 {
	sum := sha1.Sum([]byte(sfld))
	return int64(binary.BigEndian.Uint32(sum[:4]))
}

func marshalAnkiConf(deckID int64) (string, error) {
	conf := map[string]any{
		"activeDecks":    []int64{deckID},
		"curDeck":        deckID,
		"newSpread":      0,
		"collapseTime":   1200,
		"timeLim":        0,
		"estTimes":       true,
		"dueCounts":      true,
		"curModel":       strconv.FormatInt(ankiBasicModelID, 10),
		"nextPos":        1,
		"sortType":       "noteFld",
		"sortBackwards":  false,
		"addToCur":       true,
	}
	b, err := json.Marshal(conf)
	return string(b), err
}

func marshalAnkiModels(deckID, nowSec int64) (string, error) {
	modelKey := strconv.FormatInt(ankiBasicModelID, 10)
	models := map[string]any{
		modelKey: map[string]any{
			"id":        ankiBasicModelID,
			"name":      "Basic",
			"type":      0,
			"mod":       nowSec,
			"usn":       -1,
			"sortf":     0,
			"did":       deckID,
			"css":       ".card { font-family: sans-serif; font-size: 18px; text-align: left; }",
			"latexPre":  "",
			"latexPost": "",
			"tags":      []string{},
			"flds": []map[string]any{
				{"name": "Front", "ord": 0, "sticky": false, "rtl": false, "font": "Arial", "size": 18, "media": []string{}},
				{"name": "Back", "ord": 1, "sticky": false, "rtl": false, "font": "Arial", "size": 18, "media": []string{}},
			},
			"tmpls": []map[string]any{
				{
					"name":  "Card 1",
					"ord":   0,
					"qfmt":  "{{Front}}",
					"afmt":  "{{FrontSide}}<hr id=answer>{{Back}}",
					"bqfmt": "",
					"bafmt": "",
					"did":   nil,
				},
			},
		},
	}
	b, err := json.Marshal(models)
	return string(b), err
}

func marshalAnkiDecks(deckID int64, deckName string, nowSec int64) (string, error) {
	deckKey := strconv.FormatInt(deckID, 10)
	decks := map[string]any{
		"1": map[string]any{
			"id":          1,
			"name":        "Default",
			"mod":         nowSec,
			"usn":         0,
			"lrnToday":    []int{0, 0},
			"revToday":    []int{0, 0},
			"newToday":    []int{0, 0},
			"timeToday":   []int{0, 0},
			"collapsed":   false,
			"desc":        "",
			"dyn":         0,
			"conf":        1,
			"extendNew":   10,
			"extendRev":   50,
		},
		deckKey: map[string]any{
			"id":          deckID,
			"name":        deckName,
			"mod":         nowSec,
			"usn":         -1,
			"lrnToday":    []int{0, 0},
			"revToday":    []int{0, 0},
			"newToday":    []int{0, 0},
			"timeToday":   []int{0, 0},
			"collapsed":   false,
			"desc":        "Generated by Shelfd",
			"dyn":         0,
			"conf":        1,
			"extendNew":   10,
			"extendRev":   50,
		},
	}
	b, err := json.Marshal(decks)
	return string(b), err
}

func marshalAnkiDconf(nowSec int64) (string, error) {
	dconf := map[string]any{
		"1": map[string]any{
			"id":       1,
			"name":     "Default",
			"mod":      nowSec,
			"usn":      0,
			"maxTaken": 60,
			"autoplay": true,
			"timer":    0,
			"replayq":  true,
			"new": map[string]any{
				"bury":          true,
				"delays":        []int{1, 10},
				"initialFactor": 2500,
				"ints":          []int{1, 4, 7},
				"order":         1,
				"perDay":        20,
			},
			"rev": map[string]any{
				"bury":     true,
				"ease4":    1.3,
				"fuzz":     0.05,
				"ivlFct":   1,
				"maxIvl":   36500,
				"perDay":   200,
			},
			"lapse": map[string]any{
				"delays":      []int{10},
				"leechAction": 0,
				"leechFails":  8,
				"minInt":      1,
				"mult":        0,
			},
		},
	}
	b, err := json.Marshal(dconf)
	return string(b), err
}
