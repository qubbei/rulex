package vendor3rd

import (
	"github.com/i4de/rulex/archsupport"
)

/*
*
* 跨平台支持
*
 */
func EEKIT_GPIOSet(value, pin int) (bool, error) {
	return archsupport.EEKIT_GPIOSet(value, pin)
}
func EEKIT_GPIOGet(pin int) (int, error) {
	return archsupport.EEKIT_GPIOGet(pin)
}
