package upload

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
)

func EncryptFile(sourcePath, artifactPath string) (base64Key string, encryptedHash string, err error) {
	in, err := os.Open(sourcePath)
	if err != nil {
		return "", "", err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o700); err != nil {
		return "", "", err
	}
	out, err := os.Create(artifactPath)
	if err != nil {
		return "", "", err
	}
	defer func() {
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
	}()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	h := sha256.New()
	mw := io.MultiWriter(out, h)
	buf := make([]byte, ChunkSize)
	for {
		n, readErr := io.ReadFull(in, buf)
		if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
			return "", "", readErr
		}
		if n > 0 {
			nonce := make([]byte, gcm.NonceSize())
			if _, err := rand.Read(nonce); err != nil {
				return "", "", err
			}
			sealed := gcm.Seal(nil, nonce, buf[:n], nil)
			chunkLen := uint64(len(nonce) + len(sealed))
			var prefix [8]byte
			binary.BigEndian.PutUint64(prefix[:], chunkLen)
			if _, err := mw.Write(prefix[:]); err != nil {
				return "", "", err
			}
			if _, err := mw.Write(nonce); err != nil {
				return "", "", err
			}
			if _, err := mw.Write(sealed); err != nil {
				return "", "", err
			}
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
	}
	return base64.StdEncoding.EncodeToString(key), hex.EncodeToString(h.Sum(nil)), nil
}
