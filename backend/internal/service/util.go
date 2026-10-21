package service

import "strconv"

func uintToStr(u uint) string {
	return strconv.FormatUint(uint64(u), 10)
}
