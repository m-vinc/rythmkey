package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/urfave/cli/v2"
)

type CharTiming struct {
	Timing time.Duration
	Char   byte
}

func ParseRythmkey(rks string) (Rythmkey, error) {
	if len(rks) == 0 {
		return nil, errors.New("empty rythmkey")
	}

	rk := Rythmkey{}

	if rks[0] != 't' {
		return nil, errors.New("rythmkey char timing must start with a t")
	}

	var ct *CharTiming

	for i := 0; i < len(rks); i++ {
		c := rks[i]

		if ct == nil && c != 't' {
			return nil, errors.New("bad chartiming start")
		}

		if ct == nil && c == 't' {
			i += 1

			ct = &CharTiming{}
			j := 0
			for ; j < len(rks); j++ {
				if rks[i+j] >= '0' && rks[i+j] <= '9' {
					continue
				}

				timing, err := strconv.ParseInt(rks[i:i+j], 10, 64)
				if err != nil {
					return nil, err
				}

				ct.Timing = time.Duration(timing)
				break
			}

			if i+j > len(rks) {
				return nil, errors.New("missing data after timing")
			}

			ct.Char = rks[i+j]
			rk = append(rk, ct)
			ct = nil

			i += j
		}
	}

	return rk, nil
}

// 0     x    y          z
// t<c><timing><t><c><timing>
type Rythmkey []*CharTiming

func (rk *Rythmkey) Read() error {
	exec.Command("stty", "-f", "/dev/tty", "cbreak", "min", "1").Run()
	exec.Command("stty", "-f", "/dev/tty", "-echo").Run()
	defer exec.Command("stty", "-f", "/dev/tty", "echo").Run()

	buf := make([]byte, 1)

	took := time.Duration(0)
	for {
		now := time.Now()

		c, err := os.Stdin.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if buf[0] == '\n' {
			break
		}

		if len(*rk) == 0 {
			took = 0
		} else {
			took = time.Since(now)
		}

		log.Printf("get char [%c] %+v in %+v (micro: %d, milli:%s, dec:%d, hex:%X)", buf[0], c, took.Microseconds(), took.Milliseconds(), took, took, took)
		*rk = append(*rk, &CharTiming{
			Timing: time.Duration(took.Microseconds() / 1000),
			Char:   buf[0],
		})
	}

	return nil
}

func (rythmkey Rythmkey) Encode() string {
	encoded := ""

	for _, ct := range rythmkey {
		encoded += "t" + strconv.FormatInt(int64(ct.Timing), 10) + string(ct.Char)
	}

	return encoded
}

func (rythmkey Rythmkey) Hash(salt int) (string, error) {
	rk := Rythmkey{}
	for _, ct := range rythmkey {
		saltedTiming := time.Duration((((int(ct.Timing) + salt) / salt) * salt))
		rk = append(rk, &CharTiming{Char: ct.Char, Timing: saltedTiming})
	}

	h := sha256.New()

	srk := rk.Encode()
	log.Printf("rk: %s, salted rk: %s", rythmkey.Encode(), srk)
	_, err := h.Write([]byte(srk))
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (rythmkey Rythmkey) String() string {
	str := ""
	for _, pc := range rythmkey {
		str += fmt.Sprintf("%c(%d)", pc.Char, pc.Timing)
	}

	return fmt.Sprintf("%s", str)
}

func main() {
	app := &cli.App{
		Name:  "rythmkey",
		Usage: "make your password more in rythm",
		Commands: []*cli.Command{
			{
				Name: "read",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "hash",
						Value: false,
						Usage: "hash to resulting rythmkey",
					}, &cli.IntFlag{
						Name:  "salt",
						Value: 20,
						Usage: "timing salt",
					},
				},
				Aliases: []string{"r"},
				Usage:   "read a rythmkey from your terminal emulator",
				Action: func(cCtx *cli.Context) error {
					rk := Rythmkey{}

					err := rk.Read()
					if err != nil {
						return err
					}

					hash := cCtx.Bool("hash")
					if hash {
						salt := cCtx.Int("salt")
						hrk, err := rk.Hash(salt)
						if err != nil {
							return err
						}
						fmt.Print(hrk)
						return nil
					}

					fmt.Print(rk.Encode())
					return nil
				},
			}, {
				Name: "compare",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "rythmkey",
						Value:    "",
						Usage:    "ryhtmkey to compare against",
						Required: true,
					},
				},
				Aliases: []string{"cmp"},
				Usage:   "read a rythmkey from your terminal emulator and compare it",
				Action: func(cCtx *cli.Context) error {
					rks := cCtx.String("rythmkey")
					if len(rks) == 0 {
						return errors.New("empty rythmkey")
					}

					rk, err := ParseRythmkey(rks)
					if err != nil {
						return err
					}

					rrk := Rythmkey{}
					err = rrk.Read()
					if err != nil {
						return err
					}

					fmt.Printf("compare: %+v | %+v", rk, rrk)
					return nil
				},
			}, {
				Name: "parse",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "rythmkey",
						Value:    "",
						Usage:    "ryhtmkey to parse",
						Required: true,
					},
				},
				Aliases: []string{"p"},
				Usage:   "parse a rythmkey to test it and decompose it",
				Action: func(cCtx *cli.Context) error {
					rythmkey := cCtx.String("rythmkey")
					if len(rythmkey) == 0 {
						return errors.New("empty rythmkey")
					}

					rk, err := ParseRythmkey(rythmkey)
					if err != nil {
						return err
					}

					fmt.Printf("rythmkey: %+v", rk)
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
