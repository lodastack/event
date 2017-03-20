package common

func ContainString(sl []string, v string) (int, bool) {
	for index, vv := range sl {
		if vv == v {
			return index, true
		}
	}
	return 0, false
}

func RemoveDuplicateAndEmpty(sl []string) []string {
	output := make([]string, len(sl))
	var index int
	for _, s := range sl {
		if _, ok := ContainString(output, s); !ok && s != "" {
			output[index] = s
			index++
		}
	}
	return output[:index]
}
