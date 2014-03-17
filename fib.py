def fib(i):
	print ".call", i
	if i == 0:
		return 0
	s = i + fib(i - 1)
	print ".sum", s
	return s

fib(10)
