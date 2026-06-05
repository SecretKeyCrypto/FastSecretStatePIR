CXX      ?= c++
CXXFLAGS  = -O3 -std=c++17 -Wall -Wextra

rme: rme.cpp
	$(CXX) $(CXXFLAGS) -o rme rme.cpp

# Single-threaded build for apple-to-apple comparison.
rme_st: rme.cpp
	$(CXX) -O3 -std=c++17 -Wall -Wextra -DSINGLE_THREAD -o rme_st rme.cpp

clean:
	rm -f rme rme_st

.PHONY: clean
