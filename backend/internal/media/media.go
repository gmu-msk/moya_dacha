// Package media хранит файлы пользователей — пока только аватары.
//
// Хранилище спрятано за интерфейсом Storage: в MVP файлы лежат на диске
// рядом с сервисом, и он же их раздаёт, а переезд на S3-совместимое
// хранилище будет новой реализацией этого интерфейса и не затронет
// ни контракт, ни хендлеры (docs/adr/0011-files-behind-storage-interface.md).
package media

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ErrKeyOutsideStorage — ключ ведёт за пределы хранилища. Ключи строит
// сам сервис, поэтому такое означает ошибку в коде, а не в запросе.
var ErrKeyOutsideStorage = errors.New("ключ ведёт за пределы хранилища")

// Storage хранит файлы и знает, по какой ссылке их отдают наружу.
//
// Ключ — это путь файла внутри хранилища (`avatars/6f1c….jpg`), и
// именно он лежит в базе: ссылка строится из ключа, поэтому смена
// хранилища не требует переписывать данные.
type Storage interface {
	Put(ctx context.Context, key string, content []byte) error
	Delete(ctx context.Context, key string) error

	// URL — ссылка, по которой файл доступен клиенту. Может быть
	// относительной: клиент достраивает её до адреса сервиса.
	URL(key string) string

	// FileHandler раздаёт сохранённые файлы: путь, по которому их
	// вешать, и сам обработчик. Пустой путь означает, что файлы раздаёт
	// не сервис (так будет с объектным хранилищем).
	FileHandler() (string, http.Handler)
}

// Key придумывает случайное имя файла с этим расширением в этой папке.
// Имя случайное, чтобы чужой файл нельзя было угадать по ссылке.
func Key(dir, ext string) (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return path.Join(dir, hex.EncodeToString(raw)+ext), nil
}

// Disk — хранилище на локальном диске рядом с сервисом.
type Disk struct {
	dir     string
	baseURL string
}

// NewDisk создаёт хранилище в папке dir, файлы которого раздаются
// по адресам вида <baseURL>/<ключ>.
func NewDisk(dir, baseURL string) *Disk {
	return &Disk{dir: dir, baseURL: strings.TrimSuffix(baseURL, "/")}
}

func (d *Disk) Put(_ context.Context, key string, content []byte) error {
	name, err := d.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	return os.WriteFile(name, content, 0o644)
}

func (d *Disk) Delete(_ context.Context, key string) error {
	name, err := d.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (d *Disk) URL(key string) string {
	return d.baseURL + "/" + key
}

// FileHandler раздаёт файлы хранилища по тем же адресам, которые
// возвращает URL.
func (d *Disk) FileHandler() (string, http.Handler) {
	prefix := d.baseURL + "/"
	return prefix, http.StripPrefix(prefix, http.FileServer(http.Dir(d.dir)))
}

// path переводит ключ в путь на диске, не давая выйти за пределы папки.
func (d *Disk) path(key string) (string, error) {
	clean := path.Clean("/" + key)
	if clean == "/" {
		return "", ErrKeyOutsideStorage
	}
	return filepath.Join(d.dir, filepath.FromSlash(clean)), nil
}
