package cmd

import (
	"fmt"
	"net/url"
	"strings"

	"image"
	_ "image/jpeg"
	"os"

	"github.com/joeguo/tldextract"
	"github.com/sensepost/gowitness/internal/ascii"
	"github.com/sensepost/gowitness/pkg/database"
	"github.com/sensepost/gowitness/pkg/log"
	"github.com/sensepost/gowitness/pkg/models"
	"github.com/spf13/cobra"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Use gowitness plugins",
	Long:  ascii.LogoHelp(`Use gowitness plugins`),
}

type NetworkLogResult struct {
	URL string
	Log string
}

var commonExtensions []string = []string{
	".png",
	".jpg",
	".jpeg",
	".svg",
	".js",
	".css",
	".ttf",
	".woff",
	".woff2",
	".webp",
}

func isFile(path string) bool {
	for _, c := range commonExtensions {
		if strings.Contains(path, c) {
			return true
		}
	}
	return false
}

func GenerateParentPaths(rawURL string) (results []string, err error) {
	if rawURL == "about:blank" {
		return
	}
	if strings.HasPrefix(rawURL, "blob:") {
		rawURL = strings.Replace(rawURL, "blob:", "", 1)
	}
	if !strings.HasPrefix(rawURL, "http") {
		return
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return
	}

	// Get the path without query/fragment
	p := parsed.Path
	// Ensure it ends without a slash for consistent splitting
	p = strings.TrimSuffix(p, "/")

	segments := strings.Split(p, "/")

	for i := len(segments); i > 0; i-- {
		/*if isFile(segments[i]) {
			continue
		}*/
		joined := strings.Join(segments[:i], "/") + "/"
		u := *parsed
		u.Path = joined
		results = append(results, strings.Split(u.String(), "?")[0])
	}

	return
}

var headersCmd = &cobra.Command{
	Use:   "headers",
	Short: "Extract all response and request headers",
	Long:  ascii.LogoHelp(`Take all the visited hosts, loop through loaded resources extract header names and values`),
	Run: func(cmd *cobra.Command, args []string) {
		c, err := database.Connection(opts.Writer.DbURI, true, false)
		if err != nil {
			log.Fatal("failed to connect to database", "error", err)
		}
		var results []models.Header
		tx := c.Find(&results)
		fmt.Println(tx.Error)
		fmt.Println(len(results))
		for _, h := range results {
			fmt.Println(h.Key)
		}
	},
}

var pathsCmd = &cobra.Command{
	Use:   "paths",
	Short: "Extract unique in scope paths for all visited hosts",
	Long:  ascii.LogoHelp(`Take all the visited hosts, loop through loaded resources, pick the ones in scope, enumerate over the possible paths`),
	Run: func(cmd *cobra.Command, args []string) {
		c, err := database.Connection(opts.Writer.DbURI, true, false)
		if err != nil {
			log.Fatal("failed to connect to database", "error", err)
		}
		var result []NetworkLogResult
		c.Raw("SELECT DISTINCT r.url as URL, nl.url as Log FROM results r JOIN network_logs nl ON r.id = nl.result_id").Scan(&result)
		tldExtractor, _ := tldextract.New("tld-cache.txt", false)
		tldCache := map[string]string{}
		for _, r := range result {
			// Extract the registered domain (eTLD+1)
			root, ok := tldCache[r.URL]
			if !ok {
				e := tldExtractor.Extract(r.URL)
				tldCache[r.URL] = e.Root + e.Tld
			}
			l := tldExtractor.Extract(r.Log)
			if l.Root+l.Tld != root {
				continue
			}
			p, _ := GenerateParentPaths(r.Log)
			for _, pp := range p {
				fmt.Println(pp)
			}
		}
	},
}

var qrCmd = &cobra.Command{
	Use:   "qr",
	Short: "Extract detected QR codes in screenshots",
	Long:  "Extract detected QR codes in screenshots",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := database.Connection(opts.Writer.DbURI, true, false)
		if err != nil {
			log.Fatal("failed to connect to database", "error", err)
		}
		var results []models.Result
		c.Find(&results)
		qrReader := qrcode.NewQRCodeReader()
		for _, r := range results {
			file, _ := os.Open(opts.Scan.ScreenshotPath + "/" + r.Filename)
			img, _, err := image.Decode(file)
			if err != nil {
				continue
			}
			bmp, _ := gozxing.NewBinaryBitmapFromImage(img)
			result, _ := qrReader.Decode(bmp, nil)
			if result == nil {
				continue
			}

			fmt.Println(r.URL, result)
		}
	},
}

func init() {
	rootCmd.AddCommand(pluginCmd)
	pluginCmd.PersistentFlags().StringVar(&opts.Writer.DbURI, "write-db-uri", "sqlite://gowitness.sqlite3", "The database URI to use. Supports SQLite, Postgres, and MySQL (e.g., postgres://user:pass@host:port/db)")
	pluginCmd.AddCommand(pathsCmd)
	pluginCmd.AddCommand(qrCmd)
	pluginCmd.AddCommand(headersCmd)

}
