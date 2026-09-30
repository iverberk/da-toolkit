package analysis

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
	"strconv"
)

func EncryptFile(sourcePath, artifactPath string) (base64Key string, encryptedHash string, err error) {
	key, err := GenerateAESKey()
	if err != nil {
		return "", "", err
	}
	hash, err := EncryptFileWithKey(sourcePath, artifactPath, key)
	if err != nil {
		return "", "", err
	}
	return key, hash, nil
}

func GenerateAESKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

func EncryptFileWithKey(sourcePath, artifactPath, base64Key string) (encryptedHash string, err error) {
	in, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o700); err != nil {
		return "", err
	}
	out, err := os.Create(artifactPath)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
	}()
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return "", err
	}
	if len(key) != 32 {
		return "", aes.KeySizeError(len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	mw := io.MultiWriter(out, h)
	buf := make([]byte, ChunkSize)
	for {
		n, readErr := io.ReadFull(in, buf)
		if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
			return "", readErr
		}
		if n > 0 {
			nonce := make([]byte, gcm.NonceSize())
			if _, err := rand.Read(nonce); err != nil {
				return "", err
			}
			sealed := gcm.Seal(nil, nonce, buf[:n], nil)
			chunkLen := uint64(len(nonce) + len(sealed))
			var prefix [8]byte
			binary.BigEndian.PutUint64(prefix[:], chunkLen)
			if _, err := mw.Write(prefix[:]); err != nil {
				return "", err
			}
			if _, err := mw.Write(nonce); err != nil {
				return "", err
			}
			if _, err := mw.Write(sealed); err != nil {
				return "", err
			}
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

const maxEncryptedChunkLength = 1024 * 1024 * 1024

func DecryptFile(sourcePath, outputPath, base64Key string) (err error) {
	in, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
	}()
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return err
	}
	if len(key) != 32 {
		return aes.KeySizeError(len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	var prefix [8]byte
	for {
		if _, err := io.ReadFull(in, prefix[:]); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		chunkLen := binary.BigEndian.Uint64(prefix[:])
		minimumChunkLen := uint64(gcm.NonceSize() + gcm.Overhead())
		if chunkLen < minimumChunkLen || chunkLen > maxEncryptedChunkLength {
			return errInvalidEncryptedChunkLength(chunkLen)
		}
		chunk := make([]byte, chunkLen)
		if _, err := io.ReadFull(in, chunk); err != nil {
			return err
		}
		nonce := chunk[:gcm.NonceSize()]
		sealed := chunk[gcm.NonceSize():]
		plain, err := gcm.Open(nil, nonce, sealed, nil)
		if err != nil {
			return err
		}
		if _, err := out.Write(plain); err != nil {
			return err
		}
	}
}

type errInvalidEncryptedChunkLength uint64

func (e errInvalidEncryptedChunkLength) Error() string {
	return "invalid encrypted chunk length: " + strconv.FormatUint(uint64(e), 10)
}
