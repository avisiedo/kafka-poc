package utils

func DieOnError(err error) {
	if err != nil {
		panic(err.Error())
	}
}
