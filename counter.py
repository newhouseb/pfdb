import time
import random

i = 0
while 1:
  print ".debug.counter", i
  print ".debug.random", random.random()
  i += 1
  time.sleep(0.5)
