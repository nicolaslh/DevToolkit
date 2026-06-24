// Package mockgen generates batches of random mock data (R4).
package mockgen

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

const MaxCount = 10000

// Kind enumerates supported data types.
type Kind string

const (
	Name     Kind = "name"
	Address  Kind = "address"
	Phone    Kind = "phone"
	Email    Kind = "email"
	BankCard Kind = "bankcard"
	Lorem    Kind = "lorem"
)

var (
	surnames  = []string{"王", "李", "张", "刘", "陈", "杨", "赵", "黄", "周", "吴", "徐", "孙", "马", "朱", "胡", "林", "郭", "何", "高", "罗"}
	givenName = []string{"伟", "芳", "娜", "秀英", "敏", "静", "丽", "强", "磊", "军", "洋", "勇", "艳", "杰", "娟", "涛", "明", "超", "霞", "平"}
	cities    = []string{"北京市", "上海市", "广州市", "深圳市", "杭州市", "成都市", "武汉市", "南京市", "西安市", "重庆市"}
	districts = []string{"朝阳区", "海淀区", "浦东新区", "天河区", "南山区", "西湖区", "武侯区", "江汉区", "鼓楼区", "雁塔区"}
	streets   = []string{"人民路", "中山路", "解放路", "建设路", "和平街", "文化路", "新华街", "胜利路", "光明街", "幸福路"}
	emailDom  = []string{"gmail.com", "outlook.com", "qq.com", "163.com", "126.com", "foxmail.com", "example.com"}
	loremWord = []string{"lorem", "ipsum", "dolor", "sit", "amet", "consectetur", "adipiscing", "elit", "sed", "do",
		"eiusmod", "tempor", "incididunt", "ut", "labore", "et", "dolore", "magna", "aliqua", "enim"}
	phonePrefix = []string{"139", "138", "137", "136", "135", "188", "187", "186", "159", "158", "150", "151", "176", "177", "178"}
	// Common bank card BINs (prefixes) for realistic-looking numbers.
	cardBIN = []string{"622202", "621700", "622848", "625965", "436742", "356896"}
	letters = "abcdefghijklmnopqrstuvwxyz"
)

func randInt(n int) int {
	if n <= 0 {
		return 0
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0
	}
	return int(v.Int64())
}

func pick(list []string) string { return list[randInt(len(list))] }

func randDigits(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(byte('0' + randInt(10)))
	}
	return b.String()
}

// Generate produces count items of the given kind (R4.1–4.6).
func Generate(kind Kind, count int) ([]string, error) {
	if count < 1 || count > MaxCount {
		return nil, apperr.Newf(apperr.InvalidInput, "生成数量须为 1 至 %d 的整数", MaxCount)
	}
	gen, ok := generators[kind]
	if !ok {
		return nil, apperr.New(apperr.InvalidInput, "请先选择有效的数据类型")
	}
	out := make([]string, count)
	for i := 0; i < count; i++ {
		out[i] = gen()
	}
	return out, nil
}

var generators = map[Kind]func() string{
	Name:     genName,
	Address:  genAddress,
	Phone:    genPhone,
	Email:    genEmail,
	BankCard: genBankCard,
	Lorem:    genLorem,
}

func genName() string { return pick(surnames) + pick(givenName) }

func genAddress() string {
	return fmt.Sprintf("%s%s%s%d号", pick(cities), pick(districts), pick(streets), randInt(999)+1)
}

// genPhone returns an 11-digit number starting with 1 (R4.4).
func genPhone() string { return pick(phonePrefix) + randDigits(8) }

func genEmail() string {
	n := randInt(6) + 4
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(letters[randInt(len(letters))])
	}
	return b.String() + "@" + pick(emailDom)
}

// genBankCard returns a Luhn-valid card number (R4 bank card validity).
func genBankCard() string {
	bin := pick(cardBIN)
	body := bin + randDigits(16-len(bin)-1) // total 16 digits incl. check digit
	return body + string(rune('0'+luhnCheckDigit(body)))
}

func genLorem() string {
	n := randInt(20) + 8
	words := make([]string, n)
	for i := 0; i < n; i++ {
		words[i] = loremWord[randInt(len(loremWord))]
	}
	s := strings.Join(words, " ")
	return strings.ToUpper(s[:1]) + s[1:] + "."
}

// luhnCheckDigit computes the Luhn check digit for the given numeric string.
func luhnCheckDigit(number string) int {
	sum := 0
	double := true // next digit from the right (the check position) doubles
	for i := len(number) - 1; i >= 0; i-- {
		d := int(number[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return (10 - (sum % 10)) % 10
}

// LuhnValid reports whether a full number passes the Luhn checksum (test helper).
func LuhnValid(number string) bool {
	sum := 0
	double := false
	for i := len(number) - 1; i >= 0; i-- {
		d := int(number[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}
