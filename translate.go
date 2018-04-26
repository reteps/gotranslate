package gotranslate

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/liudanking/goutil/netutil"
	gcache "github.com/patrickmn/go-cache"
)

const (
	TRANSLATE_COM_ADDR = "https://translate.google.com"
	TRANSLATE_CN_ADDR  = "http://translate.google.cn"
)

// ISO839-1 https://cloud.google.com/translate/docs/languages
var _supportedLangs = map[string]string{	"af":"Afrikaans","sq":"Albanian","am":"Amharic","ar":"Arabic",
"hy":"Armenian","az":"Azeerbaijani","eu":"Basque","be":"Belarusian","bn":"Bengali",
"bs":"Bosnian","bg":"Bulgarian","ca":"Catalan","ceb":"Cebuano","zh-CN":"Chinese (Simplified)",
"zh-TW":"Chinese (Traditional)","co":"Corsican","hr":"Croatian",
"cs":"Czech","da":"Danish","nl":"Dutch","en":"English","eo":"Esperanto","et":"Estonian","fi":"Finnish","fr":"French",
"fy":"Frisian","gl":"Galician","ka":"Georgian","de":"German","el":"Greek","gu":"Gujarati","ht":"Haitian Creole",
"ha":"Hausa","haw":"Hawaiian","iw":"Hebrew","hi":"Hindi","hmn":"Hmong","hu":"Hungarian","is":"Icelandic","ig":"Igbo",
"id":"Indonesian","ga":"Irish","it":"Italian","ja":"Japanese","jw":"Javanese","kn":"Kannada","kk":"Kazakh","km":"Khmer",
"ko":"Korean","ku":"Kurdish","ky":"Kyrgyz","lo":"Lao","la":"Latin","lv":"Latvian","lt":"Lithuanian","lb":"Luxembourgish",
"mk":"Macedonian","mg":"Malagasy","ms":"Malay","ml":"Malayalam","mt":"Maltese","mi":"Maori","mr":"Marathi",
"mn":"Mongolian","my":"Myanmar (Burmese)","ne":"Nepali","no":"Norwegian","ny":"Nyanja (Chichewa)","ps":"Pashto",
"fa":"Persian","pl":"Polish","pt":"Portuguese","pa":"Punjabi","ro":"Romanian","ru":"Russian","sm":"Samoan",
"gd":"Scots Gaelic","sr":"Serbian","st":"Sesotho","sn":"Shona","sd":"Sindhi","si":"Sinhalese","sk":"Slovak",
"sl":"Slovenian","so":"Somali","es":"Spanish","su":"Sundanese","sw":"Swahili","sv":"Swedish",
"tl":"Tagalog (Filipino)","tg":"Tajik","ta":"Tamil","te":"Telugu","th":"Thai","tr":"Turkish","uk":"Ukrainian","ur":"Urdu",
"uz":"Uzbek","vi":"Vietnamese","cy":"Welsh","xh":"Xhosa","yi":"Yiddish","yo":"Yoruba","zu":"Zulu"}

var defaultGTranslate *GTranslate

func init() {
	defaultGTranslate, _ = New(TRANSLATE_CN_ADDR, nil)
}

type GTranslate struct {
	srvAddr string
	proxy   func(r *http.Request) (*url.URL, error)
	cache   *gcache.Cache
}

type typeTKK struct {
	h1 int
	h2 int
}
func Language(code string) string {
	return _supportedLangs[code]
}
func New(addr string, proxy func(r *http.Request) (*url.URL, error)) (*GTranslate, error) {
	if addr != TRANSLATE_CN_ADDR && addr != TRANSLATE_COM_ADDR {
		return nil, errors.New("addr not supported")
	}
	return &GTranslate{
		srvAddr: addr,
		proxy:   proxy,
		cache:   gcache.New(10*time.Minute, 5*time.Minute),
	}, nil
}

type TranslateRet struct {
	Sentences []struct {
		Trans   string `json:"trans"`
		Orig    string `json:"orig"`
		Backend int    `json:"backend"`
	} `json:"sentences"`
	Src        string  `json:"src"`
	Confidence float64 `json:"confidence"`
	LdResult   struct {
		Srclangs            []string  `json:"srclangs"`
		SrclangsConfidences []float64 `json:"srclangs_confidences"`
		ExtendedSrclangs    []string  `json:"extended_srclangs"`
	} `json:"ld_result"`
}

// Translate translate q from sl to tl using default GTranslate
func Translate(sl, tl, q string) (*TranslateRet, error) {
	return defaultGTranslate.Translate(sl, tl, q)
}

// SimpleTranslate translate q to tl without test q sentences
func SimpleTranslate(sl, tl, q string) (string, error) {
	return defaultGTranslate.SimpleTranslate(sl, tl, q)
}

func (gt *GTranslate) Translate(sl, tl, q string) (*TranslateRet, error) {
	if sl != "auto" && _, exists := _supportedLangs[sl]; !exists {
			return nil, errors.New("source language not supported")
	}

	if _, exists := _supportedLangs[tl]; !exists {
		return nil, errors.New("target language not supported")
	}

	tkk, err := gt.getTKK()
	if err != nil {
		log.Printf("get tkk error:%v", err)
		return nil, err
	}
	h1, h2 := tkk.h1, tkk.h2

	tkstr := tk(h1, h2, q)

	// https://translate.google.com/translate_a/single?client=t&sl=zh-CN&tl=zh-TW&hl=zh-CN&dt=at&dt=bd&dt=ex&dt=ld&dt=md&dt=qca&dt=rw&dt=rm&dt=ss&dt=t&ie=UTF-8&oe=UTF-8&otf=2&ssel=0&tsel=0&kc=1&tk=%s&q=%s
	addr := fmt.Sprintf("%s/translate_a/single", gt.srvAddr)
	data, err := gt.httpRequest("GET", addr, gt.reqParams(sl, tl, tkstr, q))

	ret := &TranslateRet{}
	err = json.Unmarshal(data, ret)
	ret.
	return ret, err
}

func (gt *GTranslate) SimpleTranslate(sl, tl, q string) (string, error) {
	rsp, err := gt.Translate(sl, tl, q)
	if err != nil {
		return "", err
	}
	s := ""
	for _, sentence := range rsp.Sentences {
		s += sentence.Trans
	}
	return s, nil
}

func (gt *GTranslate) reqParams(sl, tl, tk, q string) map[string]interface{} {
	return map[string]interface{}{
		"client": "t",     // or gtx
		"sl":     sl,      // source language
		"tl":     tl,      // translated language
		"dj":     1,       // ensure return json is GoogleRes structure
		"ie":     "UTF-8", // input string encoding
		"oe":     "UTF-8", // output string encoding
		"tk":     tk,
		"q":      q,
		"dt":     []string{"t", "bd"}, // a list to add content to return json
		// possible dt values: correspond return json key
		// t: sentences
		// rm: sentences[1]
		// bd: dict
		// at: alternative_translations
		// ss: synsets
		// rw: related_words
		// ex: examples
		// ld: ld_result
	}
}

func (gt *GTranslate) httpRequest(method, addr string, params map[string]interface{}) ([]byte, error) {

	data, code, err := netutil.DefaultHttpClient().RequestForm(method, addr, params).UserAgent(netutil.UA_SAFARI).Proxy(gt.proxy).DoByte()
	if err != nil {
		log.Printf("http request failed:[%d] %v", code, err)
	}
	return data, err
}
