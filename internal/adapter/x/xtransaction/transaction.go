package xtransaction

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Client generates x.com's x-client-transaction-id header. Ported from
// twikit's ClientTransaction. Construct via New, then call GenerateTransactionID
// per request.
type Client struct {
	key            string
	keyBytes       []int
	animationKey   string
	rowIndex       int
	keyByteIndices []int
}

const (
	additionalRandomNumber = 3
	defaultKeyword         = "obfiowerehiring"
	// xEpochMillis is the x.com transaction epoch (2023-05-01T07:00:00Z) in ms.
	xEpochSeconds = 1682924400
)

var (
	// Chunk id for ondemand.s is listed separately from its file hash in the
	// current webpack layout; resolve in two steps.
	onDemandChunkRegex = regexp.MustCompile(`[,{](\d+):["']ondemand\.s["']`)
	indicesRegex       = regexp.MustCompile(`\(\w{1}\[(\d{1,2})\],\s*16\)`)
	nonDigitRegex      = regexp.MustCompile(`[^\d]+`)
	dotDashRegex       = regexp.MustCompile(`[.-]`)
)

// New builds a transaction Client from the x.com home page HTML and the
// matching ondemand.s.<hash>a.js file contents. The caller fetches both
// (home page while authenticated) and passes the raw bytes.
func New(homePageHTML, onDemandJS string) (*Client, error) {
	c := &Client{}

	rowIdx, keyByteIndices, err := parseIndices(onDemandJS)
	if err != nil {
		return nil, err
	}
	c.rowIndex = rowIdx
	c.keyByteIndices = keyByteIndices

	key, err := getKey(homePageHTML)
	if err != nil {
		return nil, err
	}
	c.key = key
	c.keyBytes, err = keyToBytes(key)
	if err != nil {
		return nil, err
	}

	animKey, err := c.getAnimationKey(homePageHTML)
	if err != nil {
		return nil, err
	}
	c.animationKey = animKey
	return c, nil
}

// OnDemandFileURL extracts the ondemand.s file hash from the home page and
// returns the full JS URL the caller must fetch to build a Client.
func OnDemandFileURL(homePageHTML string) (string, error) {
	m := onDemandChunkRegex.FindStringSubmatch(homePageHTML)
	if m == nil {
		return "", fmt.Errorf("ondemand.s chunk id not found in home page")
	}
	chunkID := m[1]
	hashRe := regexp.MustCompile(fmt.Sprintf(`,%s:["']([0-9a-f]+)["']`, regexp.QuoteMeta(chunkID)))
	hashMatch := hashRe.FindStringSubmatch(homePageHTML)
	if hashMatch == nil {
		return "", fmt.Errorf("ondemand.s hash not found for chunk id %s", chunkID)
	}
	return fmt.Sprintf("https://abs.twimg.com/responsive-web/client-web/ondemand.s.%sa.js", hashMatch[1]), nil
}

func parseIndices(onDemandJS string) (int, []int, error) {
	matches := indicesRegex.FindAllStringSubmatch(onDemandJS, -1)
	if len(matches) == 0 {
		return 0, nil, fmt.Errorf("couldn't get KEY_BYTE indices")
	}
	idx := make([]int, 0, len(matches))
	for _, m := range matches {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, nil, err
		}
		idx = append(idx, n)
	}
	return idx[0], idx[1:], nil
}

// getKey reads the twitter-site-verification meta content from the home page.
func getKey(homePageHTML string) (string, error) {
	root, err := html.Parse(strings.NewReader(homePageHTML))
	if err != nil {
		return "", err
	}
	var content string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if content != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "meta" {
			var name, c string
			for _, a := range n.Attr {
				if a.Key == "name" {
					name = a.Val
				}
				if a.Key == "content" {
					c = a.Val
				}
			}
			if name == "twitter-site-verification" {
				content = c
				return
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(root)
	if content == "" {
		return "", fmt.Errorf("couldn't get key from page source")
	}
	return content, nil
}

func keyToBytes(key string) ([]int, error) {
	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}
	out := make([]int, len(raw))
	for i, b := range raw {
		out[i] = int(b)
	}
	return out, nil
}

// getFrameDs returns the path "d" attribute strings of the loading-x-anim
// frames from the home page, in document order.
func getFrameDs(homePageHTML string) ([]string, error) {
	root, err := html.Parse(strings.NewReader(homePageHTML))
	if err != nil {
		return nil, err
	}
	// Collect frames whose id starts with loading-x-anim, then within each the
	// path element's "d" attr: list(list(frame.children)[0].children)[1].d
	var ds []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for _, a := range n.Attr {
				if a.Key == "id" && strings.HasPrefix(a.Val, "loading-x-anim") {
					if d, ok := frameD(n); ok {
						ds = append(ds, d)
					}
				}
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(root)
	if len(ds) == 0 {
		return nil, fmt.Errorf("no loading-x-anim frames found")
	}
	return ds, nil
}

// frameD mirrors list(list(frame.children)[0].children)[1].get("d"):
// first element child of frame, then its second element child's "d" attr.
func frameD(frame *html.Node) (string, bool) {
	first := firstElementChild(frame)
	if first == nil {
		return "", false
	}
	elems := elementChildren(first)
	if len(elems) < 2 {
		return "", false
	}
	for _, a := range elems[1].Attr {
		if a.Key == "d" {
			return a.Val, true
		}
	}
	return "", false
}

func firstElementChild(n *html.Node) *html.Node {
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.ElementNode {
			return ch
		}
	}
	return nil
}

func elementChildren(n *html.Node) []*html.Node {
	var out []*html.Node
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.ElementNode {
			out = append(out, ch)
		}
	}
	return out
}

// get2DArray parses the frame path "d" for the frame selected by keyBytes[5]%4:
// strip leading 9 chars, split on "C", each group → ints.
func (c *Client) get2DArray(homePageHTML string) ([][]int, error) {
	ds, err := getFrameDs(homePageHTML)
	if err != nil {
		return nil, err
	}
	idx := c.keyBytes[5] % 4
	if idx >= len(ds) {
		return nil, fmt.Errorf("frame index %d out of range (%d frames)", idx, len(ds))
	}
	d := ds[idx]
	if len(d) < 9 {
		return nil, fmt.Errorf("frame path too short")
	}
	groups := strings.Split(d[9:], "C")
	out := make([][]int, 0, len(groups))
	for _, g := range groups {
		g = strings.TrimSpace(nonDigitRegex.ReplaceAllString(g, " "))
		if g == "" {
			continue
		}
		parts := strings.Fields(g)
		row := make([]int, 0, len(parts))
		for _, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil {
				return nil, err
			}
			row = append(row, n)
		}
		out = append(out, row)
	}
	return out, nil
}

func (c *Client) getAnimationKey(homePageHTML string) (string, error) {
	const totalTime = 4096.0
	rowIndex := c.keyBytes[c.rowIndex] % 16
	frameTime := 1
	for _, i := range c.keyByteIndices {
		frameTime *= c.keyBytes[i] % 16
	}
	arr, err := c.get2DArray(homePageHTML)
	if err != nil {
		return "", err
	}
	if rowIndex >= len(arr) {
		return "", fmt.Errorf("row index %d out of range (%d rows)", rowIndex, len(arr))
	}
	frameRow := arr[rowIndex]
	targetTime := float64(frameTime) / totalTime
	return animate(frameRow, targetTime), nil
}

func animate(frames []int, targetTime float64) string {
	fromColor := []float64{float64(frames[0]), float64(frames[1]), float64(frames[2]), 1}
	toColor := []float64{float64(frames[3]), float64(frames[4]), float64(frames[5]), 1}
	fromRotation := []float64{0.0}
	toRotation := []float64{solve(float64(frames[6]), 60.0, 360.0, true)}

	rest := frames[7:]
	curves := make([]float64, len(rest))
	for i, item := range rest {
		curves[i] = solve(float64(item), isOdd(i), 1.0, false)
	}
	cub := newCubic(curves)
	val := cub.getValue(targetTime)

	color := interpolate(fromColor, toColor, val)
	for i := range color {
		if color[i] < 0 {
			color[i] = 0
		}
	}
	rotation := interpolate(fromRotation, toRotation, val)
	matrix := convertRotationToMatrix(rotation[0])

	var sb strings.Builder
	for i := 0; i < len(color)-1; i++ {
		sb.WriteString(strconv.FormatInt(int64(math.Round(color[i])), 16))
	}
	for _, value := range matrix {
		rounded := math.Round(value*100) / 100
		if rounded < 0 {
			rounded = -rounded
		}
		hexValue := floatToHex(rounded)
		switch {
		case strings.HasPrefix(hexValue, "."):
			sb.WriteString(strings.ToLower("0" + hexValue))
		case hexValue == "":
			sb.WriteString("0")
		default:
			sb.WriteString(hexValue)
		}
	}
	sb.WriteString("00")
	return dotDashRegex.ReplaceAllString(sb.String(), "")
}

// GenerateTransactionID returns the x-client-transaction-id value for a given
// HTTP method and URL path (e.g. "/i/api/graphql/<id>/CreateTweet").
func (c *Client) GenerateTransactionID(method, path string) string {
	timeNow := int64(math.Floor(float64(time.Now().UnixMilli()-xEpochSeconds*1000) / 1000))
	timeNowBytes := make([]int, 4)
	for i := 0; i < 4; i++ {
		timeNowBytes[i] = int((timeNow >> (uint(i) * 8)) & 0xFF)
	}

	hashInput := fmt.Sprintf("%s!%s!%d%s%s", method, path, timeNow, defaultKeyword, c.animationKey)
	sum := sha256.Sum256([]byte(hashInput))

	bytesArr := make([]int, 0, len(c.keyBytes)+4+16+1)
	bytesArr = append(bytesArr, c.keyBytes...)
	bytesArr = append(bytesArr, timeNowBytes...)
	for i := 0; i < 16; i++ {
		bytesArr = append(bytesArr, int(sum[i]))
	}
	bytesArr = append(bytesArr, additionalRandomNumber)

	randomNum := rand.Intn(256)
	out := make([]byte, 0, len(bytesArr)+1)
	out = append(out, byte(randomNum))
	for _, item := range bytesArr {
		out = append(out, byte(item^randomNum))
	}
	return strings.TrimRight(base64.StdEncoding.EncodeToString(out), "=")
}
