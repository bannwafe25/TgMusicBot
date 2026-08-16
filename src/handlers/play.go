Berikut adalah kode sumber yang telah diterjemahkan seluruh pesan, pemberitahuan, dan string antarmukanya ke dalam bahasa Indonesia:
```go
/*
 * TgMusicBot - Telegram Music Bot
 *  Copyright (c) 2025-2026 Ashok Shau
 *
 *  Licensed under GNU GPL v3
 *  See https://github.com/AshokShau/TgMusicBot
 */

package handlers

import (
	"ashokshau/tgmusic/config"
	"ashokshau/tgmusic/src/core"
	"ashokshau/tgmusic/src/core/cache"
	"ashokshau/tgmusic/src/core/db"
	"ashokshau/tgmusic/src/core/dl"
	"ashokshau/tgmusic/src/vc"
	"fmt"
	"html"
	"strings"

	"ashokshau/tgmusic/src/utils"

	td "github.com/AshokShau/gotdbot"
)

// playHandler menangani perintah /play.
func playHandler(c *td.Client, m *td.Message) error {
	if !playMode(c, m) {
		return td.EndGroups
	}

	return handlePlay(c, m, false, false)
}

// vPlayHandler menangani perintah /vplay.
func vPlayHandler(c *td.Client, m *td.Message) error {
	if !playMode(c, m) {
		return td.EndGroups
	}

	if !config.EnableVideoPlayback {
		_, _ = m.ReplyText(c, "🎥 Pemutaran video saat ini dinonaktifkan.\n\nSeiring meningkatnya penggunaan bot, streaming video terkadang dapat menyebabkan lag dan menurunkan kualitas musik dalam obrolan suara. Untuk memastikan pengalaman mendengarkan yang lancar bagi semuanya, fitur ini dimatikan untuk sementara waktu.\n\nTerima kasih atas dukungan dan pengertian Anda ❤️", nil)
		return td.EndGroups
	}
	return handlePlay(c, m, true, false)
}

// fPlayHandler menangani perintah /fplay.
func fPlayHandler(c *td.Client, m *td.Message) error {
	if !adminMode(c, m) {
		return td.EndGroups
	}

	return handlePlay(c, m, false, true)
}

// fVPlayHandler menangani perintah /fvplay.
func fVPlayHandler(c *td.Client, m *td.Message) error {
	if !adminMode(c, m) {
		return td.EndGroups
	}

	if !config.EnableVideoPlayback {
		_, _ = m.ReplyText(c, "🎥 Pemutaran video saat ini dinonaktifkan.\n\nSeiring meningkatnya penggunaan bot, streaming video terkadang dapat menyebabkan lag dan menurunkan kualitas musik dalam obrolan suara. Untuk memastikan pengalaman mendengarkan yang lancar bagi semuanya, fitur ini dimatikan untuk sementara waktu.\n\nTerima kasih atas dukungan dan pengertian Anda ❤️", nil)
		return td.EndGroups
	}
	return handlePlay(c, m, true, true)
}

func handlePlay(c *td.Client, m *td.Message, isVideo bool, force bool) error {
	chatID := m.ChatId

	if queueLen := cache.ChatCache.GetQueueLength(chatID); queueLen > 10 {
		_, _ = m.ReplyText(c, "Antrean penuh (maksimal 10 trek). Gunakan /end untuk mengosongkan.", nil)
		return td.EndGroups
	}

	isReply := m.ReplyToMessageID() != 0
	args := Args(m)
	url := getUrl(c, m, isReply)

	rMsg := m
	var err error
	if isReply && args == "" && url == "" {
		r, err := m.GetRepliedMessage(c)
		if err == nil && r != nil {
			args = r.Text()
		}
	}

	input := coalesce(url, args)

	if strings.HasPrefix(input, "tgpl_") {
		playlist, err := db.Instance.GetPlaylist(input)
		if err != nil {
			_, err = m.ReplyText(c, "❌ Playlist tidak ditemukan.", nil)
			return err
		}

		tracks := db.ConvertSongsToTracks(playlist.Songs)
		if len(tracks) == 0 {
			_, err = m.ReplyText(c, "❌ Playlist kosong.", nil)
			return err
		}

		updater, err := m.ReplyText(c, "🔍 Mencari playlist...", nil)
		if err != nil {
			c.Logger.Warn("gagal mengirim pesan", "error", err)
			return td.EndGroups
		}

		return handleMultipleTracks(c, m, updater, tracks, chatID, isVideo, force)
	}

	if match := utils.TelegramMessageRegex.FindStringSubmatch(input); match != nil {
		rMsg, err = utils.GetMessage(c, input)
		if err != nil {
			c.Logger.Warn("gagal memproses pesan", "error", err.Error())
			_, err = m.ReplyText(c, "Tautan Telegram tidak valid.", nil)
			return err
		}
	} else if isReply {
		rMsg, err = m.GetRepliedMessage(c)
		if err != nil {
			_, err = m.ReplyText(c, "Pesan balasan tidak valid.", nil)
			return err
		}
	}

	if isValid := isValidMedia(rMsg); isValid {
		isReply = true
	}

	if url == "" && args == "" && (!isReply || !isValidMedia(rMsg)) {
		_, _ = m.ReplyText(c, "<b>Cara Penggunaan:</b>\n/play [lagu atau URL]\n\n<b>Platform yang Didukung:</b>\n- YouTube\n- Spotify\n- JioSaavn\n- Apple Music", &td.SendTextMessageOpts{ReplyMarkup: core.SupportKeyboard(), ParseMode: "HTML"})
		return td.EndGroups
	}

	updater, err := m.ReplyText(c, "🔍 Mencari dan mengunduh...", nil)
	if err != nil {
		c.Logger.Warn("gagal mengirim pesan", "error", err)
		return td.EndGroups
	}

	if isReply && isValidMedia(rMsg) {
		return handleMedia(c, m, updater, rMsg, chatID, isVideo, force)
	}

	wrapper := dl.NewDownloaderWrapper(input)
	if url != "" {
		if !wrapper.IsValid() {
			_, _ = updater.EditText(c, "URL atau platform tidak didukung.\n\n<b>Platform yang Didukung:</b>\n- YouTube\n- Spotify\n- JioSaavn\n- Apple Music", &td.EditTextMessageOpts{ReplyMarkup: core.SupportKeyboard(), ParseMode: "HTML"})
			return td.EndGroups
		}

		trackInfo, err := wrapper.GetInfo()
		if err != nil {
			_, _ = updater.EditText(c, fmt.Sprintf("❌ Gagal mengambil informasi trek: %s", err.Error()), nil)
			return td.EndGroups
		}

		if trackInfo.Results == nil || len(trackInfo.Results) == 0 {
			_, _ = updater.EditText(c, "Trek tidak ditemukan.", nil)
			return td.EndGroups
		}

		return handleUrl(c, m, updater, trackInfo, chatID, isVideo, force)
	}

	return handleTextSearch(c, m, updater, wrapper, chatID, isVideo, force)
}

// handleMedia menangani pemutaran media dari sebuah pesan.
func handleMedia(c *td.Client, m *td.Message, updater *td.Message, dlMsg *td.Message, chatId int64, isVideo bool, force bool) error {
	file, fileName := getFile(dlMsg)
	if file == nil {
		_, err := updater.EditText(c, "Tidak ditemukan media yang valid di dalam pesan.", nil)
		return err
	}

	if file.Size > config.MaxFileSize {
		_, err := updater.EditText(c, fmt.Sprintf("Ukuran file terlalu besar. Maksimal: %d MB.", config.MaxFileSize/(1024*1024)), nil)
		if err != nil {
			c.Logger.Warn("Gagal mengedit pesan", "error", err)
		}
		return nil
	}

	fileId := dlMsg.RemoteFileID()
	if _track := cache.ChatCache.GetTrackIfExists(chatId, fileId); _track != nil {
		_, err := updater.EditText(c, "Trek sudah ada di dalam antrean atau sedang diputar.", nil)
		return err
	}

	dur := utils.GetFileDur(dlMsg)
	link, err := dlMsg.GetLink(c)
	if err != nil {
		c.Logger.Warn("Gagal mendapatkan tautan file", "error", err)
		link.Link = ""
	}

	saveCache := utils.CachedTrack{
		URL: link.Link, Name: fileName, User: firstName(c, m), TrackID: fileId,
		Duration: dur, IsVideo: isVideo, Platform: utils.Telegram,
	}

	var qLen int
	if force {
		qLen = cache.ChatCache.AddSongToFront(chatId, &saveCache)
	} else {
		qLen = cache.ChatCache.AddSong(chatId, &saveCache)
	}

	if qLen > 1 {
		if force {
			_ = vc.Calls.PlayNext(c, chatId)
			_ = c.DeleteMessages(chatId, []int64{updater.Id}, &td.DeleteMessagesOpts{Revoke: true})
			return nil
		}
		escURL := html.EscapeString(saveCache.URL)
		escName := html.EscapeString(saveCache.Name)
		escUser := html.EscapeString(saveCache.User)
		queueInfo := fmt.Sprintf(
			"<u><b>Ditambahkan ke antrean: %d</b></u>\n\n<b>Judul:</b> <a href='%s'>%s</a>\n\n<b>Durasi:</b> %s menit\n<b>Diminta oleh:</b> %s",
			qLen, escURL, escName, utils.SecToMin(saveCache.Duration), escUser,
		)
		_, err := updater.EditText(c, queueInfo, &td.EditTextMessageOpts{ReplyMarkup: core.QueueMarkup(saveCache.TrackID), ParseMode: "HTML", DisableWebPagePreview: true})
		return err
	}

	file, err = dlMsg.Download(c, 1, 0, 0, true)
	if err != nil {
		cache.ChatCache.RemoveCurrentSong(chatId)
		_, err = updater.EditText(c, fmt.Sprintf("Unduhan gagal: %s", err.Error()), nil)
		return err
	}

	filePath := file.Local.Path
	if dur == 0 {
		dur = utils.GetMediaDuration(filePath)
		saveCache.Duration = dur
	}

	saveCache.FilePath = filePath

	if err = vc.Calls.PlayMedia(c, chatId, saveCache.FilePath, saveCache.IsVideo, ""); err != nil {
		cache.ChatCache.RemoveCurrentSong(chatId)
		_, err = updater.EditText(c, err.Error(), &td.EditTextMessageOpts{ParseMode: "HTML", DisableWebPagePreview: true})
		return err
	}

	escURL := html.EscapeString(saveCache.URL)
	escName := html.EscapeString(saveCache.Name)
	escUser := html.EscapeString(saveCache.User)

	nowPlaying := fmt.Sprintf(
		"<u><b>| Mulai streaming</b></u>\n\n<b>Judul:</b> <a href='%s'>%s</a>\n\n<b>Durasi:</b> %s menit\n<b>Diminta oleh:</b> %s",
		escURL, escName, utils.SecToMin(saveCache.Duration), escUser,
	)

	_, err = updater.EditText(c, nowPlaying, &td.EditTextMessageOpts{
		ParseMode:             "HTML",
		ReplyMarkup:           core.ControlButtons("play"),
		DisableWebPagePreview: true,
	})

	return err
}

// handleTextSearch menangani pencarian teks untuk sebuah lagu.
func handleTextSearch(c *td.Client, m *td.Message, updater *td.Message, wrapper *dl.DownloaderWrapper, chatId int64, isVideo bool, force bool) error {
	searchResult, err := wrapper.Search()
	if err != nil {
		_, err = updater.EditText(c, fmt.Sprintf("❌ Pencarian gagal: %s", err.Error()), nil)
		return err
	}

	if searchResult.Results == nil || len(searchResult.Results) == 0 {
		_, err = updater.EditText(c, "😕 Tidak ada hasil ditemukan. Coba kueri lain.", nil)
		return err
	}

	song := searchResult.Results[0]
	if _track := cache.ChatCache.GetTrackIfExists(chatId, song.Id); _track != nil {
		_, err := updater.EditText(c, "Trek sudah ada di dalam antrean atau sedang diputar.", nil)
		return err
	}

	return handleSingleTrack(c, m, updater, song, "", chatId, isVideo, force)
}

// handleUrl menangani pencarian URL untuk sebuah lagu.
func handleUrl(c *td.Client, m *td.Message, updater *td.Message, trackInfo utils.PlatformTracks, chatId int64, isVideo bool, force bool) error {
	if len(trackInfo.Results) == 1 {
		track := trackInfo.Results[0]
		if _track := cache.ChatCache.GetTrackIfExists(chatId, track.Id); _track != nil {
			_, err := updater.EditText(c, "Trek sudah ada di dalam antrean atau sedang diputar.", nil)
			return err
		}
		return handleSingleTrack(c, m, updater, track, "", chatId, isVideo, force)
	}

	return handleMultipleTracks(c, m, updater, trackInfo.Results, chatId, isVideo, force)
}

// handleSingleTrack menangani satu trek.
func handleSingleTrack(c *td.Client, m *td.Message, updater *td.Message, song utils.MusicTrack, filePath string, chatId int64, isVideo bool, force bool) error {
	if song.Duration > int(config.SongDurationLimit) {
		_, err := updater.EditText(c, fmt.Sprintf("Maaf, durasi lagu melebihi batas maksimal %d menit.", config.SongDurationLimit/60), nil)
		return err
	}

	saveCache := utils.CachedTrack{
		URL: song.Url, Name: song.Title, User: firstName(c, m), FilePath: filePath,
		Thumbnail: song.Thumbnail, TrackID: song.Id, Duration: song.Duration, Channel: song.Channel, Views: song.Views,
		IsVideo: isVideo, Platform: song.Platform,
	}

	var qLen int
	if force {
		qLen = cache.ChatCache.AddSongToFront(chatId, &saveCache)
	} else {
		qLen = cache.ChatCache.AddSong(chatId, &saveCache)
	}

	if qLen > 1 {
		if force {
			_ = vc.Calls.PlayNext(c, chatId)
			_ = c.DeleteMessages(chatId, []int64{updater.Id}, &td.DeleteMessagesOpts{Revoke: true})
			return nil
		}
		escURL := html.EscapeString(saveCache.URL)
		escName := html.EscapeString(saveCache.Name)
		escUser := html.EscapeString(saveCache.User)
		queueInfo := fmt.Sprintf(
			"<u><b>Ditambahkan ke antrean: %d</b></u>\n\n<b>Judul:</b> <a href='%s'>%s</a>\n\n<b>Durasi:</b> %s menit\n<b>Diminta oleh:</b> %s",
			qLen, escURL, escName, utils.SecToMin(saveCache.Duration), escUser,
		)

		_, err := updater.EditText(c, queueInfo, &td.EditTextMessageOpts{ReplyMarkup: core.QueueMarkup(saveCache.TrackID), ParseMode: "HTML", DisableWebPagePreview: true})
		return err
	}

	if saveCache.FilePath == "" {
		dlResult, err := dl.DownloadCachedTrack(&saveCache, c)
		if err != nil {
			cache.ChatCache.RemoveCurrentSong(chatId)
			_, err = updater.EditText(c, fmt.Sprintf("Unduhan gagal: %s", err.Error()), nil)
			return err
		}

		saveCache.FilePath = dlResult
	}

	if err := vc.Calls.PlayMedia(c, chatId, saveCache.FilePath, saveCache.IsVideo, ""); err != nil {
		cache.ChatCache.RemoveCurrentSong(chatId)
		_, err = updater.EditText(c, err.Error(), &td.EditTextMessageOpts{ParseMode: "HTML", DisableWebPagePreview: true})
		return err
	}

	escURLnp := html.EscapeString(saveCache.URL)
	escNamenp := html.EscapeString(saveCache.Name)
	escUsernp := html.EscapeString(saveCache.User)

	nowPlaying := fmt.Sprintf(
		"<u><b>| Mulai streaming</b></u>\n\n<b>Judul:</b> <a href='%s'>%s</a>\n\n<b>Durasi:</b> %s menit\n<b>Diminta oleh:</b> %s",
		escURLnp, escNamenp, utils.SecToMin(song.Duration), escUsernp,
	)

	_, err = updater.EditText(c, nowPlaying, &td.EditTextMessageOpts{
		ReplyMarkup:           core.ControlButtons("play"),
		ParseMode:             "HTML",
		DisableWebPagePreview: true,
	})

	if err != nil {
		c.Logger.Warn("Gagal mengedit pesan", "error", err)
		return err
	}

	return nil
}

// handleMultipleTracks menangani banyak trek sekaligus.
func handleMultipleTracks(c *td.Client, m *td.Message, updater *td.Message, tracks []utils.MusicTrack, chatId int64, isVideo bool, force bool) error {
	if len(tracks) == 0 {
		_, err := updater.EditText(c, "Tidak ada trek ditemukan.", nil)
		return err
	}

	queueHeader := "<u><b>Ditambahkan ke Antrean:</b></u>\n<blockquote expandable>\n"
	var tracksToAdd []*utils.CachedTrack
	var skippedTracks []string

	shouldPlayFirst := false
	var firstTrack *utils.CachedTrack

	for _, track := range tracks {
		if track.Duration > int(config.SongDurationLimit) {
			skippedTracks = append(skippedTracks, track.Title)
			continue
		}

		saveCache := &utils.CachedTrack{
			Name: track.Title, TrackID: track.Id, Duration: track.Duration,
			Thumbnail: track.Thumbnail, User: firstName(c, m), Platform: track.Platform,
			IsVideo: isVideo, URL: track.Url, Channel: track.Channel, Views: track.Views,
		}
		tracksToAdd = append(tracksToAdd, saveCache)
	}

	if len(tracksToAdd) == 0 {
		if len(skippedTracks) > 0 {
			_, err := updater.EditText(c, fmt.Sprintf("Semua trek dilewati (melebihi batas durasi %d menit).", config.SongDurationLimit/60), nil)
			return err
		}
		_, err := updater.EditText(c, "Tidak ada trek valid yang ditemukan.", nil)
		return err
	}

	var qLenAfter int
	var startLen int

	if force {
		qLenAfter = 0
		for i := len(tracksToAdd) - 1; i >= 0; i-- {
			qLenAfter = cache.ChatCache.AddSongToFront(chatId, tracksToAdd[i])
		}
		startLen = qLenAfter - len(tracksToAdd)
		if startLen > 0 {
			_ = vc.Calls.PlayNext(c, chatId)
			_ = c.DeleteMessages(chatId, []int64{updater.Id}, &td.DeleteMessagesOpts{Revoke: true})
			return nil
		}
	} else {
		qLenAfter = cache.ChatCache.AddSongs(chatId, tracksToAdd)
		startLen = qLenAfter - len(tracksToAdd)
	}

	if startLen == 0 {
		shouldPlayFirst = true
		firstTrack = tracksToAdd[0]
		firstTrack.Loop = 1
	}

	var sb strings.Builder
	sb.WriteString(queueHeader)

	totalDuration := 0
	for i, track := range tracksToAdd {
		currentQLen := startLen + i + 1
		escTrackName := html.EscapeString(track.Name)
		fmt.Fprintf(&sb, "<b>%d.</b> %s\n└ Durasi: %s\n",
			currentQLen, escTrackName, utils.SecToMin(track.Duration))
		totalDuration += track.Duration
	}

	sb.WriteString("</blockquote>")
	escRequester := html.EscapeString(firstName(c, m))
	queueSummary := fmt.Sprintf(
		"\n<b>Total Antrean:</b> %d\n<b>Durasi:</b> %s menit\n<b>Diminta oleh:</b> %s",
		qLenAfter, utils.SecToMin(totalDuration), escRequester,
	)

	sb.WriteString(queueSummary)
	if len(skippedTracks) > 0 {
		fmt.Fprintf(&sb, "\n\n<b>Melewati %d trek</b> (melebihi batas durasi).", len(skippedTracks))
	}

	fullMessage := sb.String()

	if len(fullMessage) > 4096 {
		fullMessage = queueSummary
	}

	if shouldPlayFirst && firstTrack != nil {
		_ = vc.Calls.PlayNext(c, chatId)
	}

	_, err := updater.EditText(c, fullMessage, &td.EditTextMessageOpts{
		ParseMode:             "HTML",
		ReplyMarkup:           core.QueueMarkup(tracksToAdd[0].TrackID),
		DisableWebPagePreview: true,
	})

	return err
}

```
