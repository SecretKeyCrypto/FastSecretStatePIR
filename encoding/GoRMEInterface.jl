include("RMEncoding.jl")
using CSV, DataFrames

q = parse(Int, ARGS[1])

if length(ARGS) >= 2
    # Parse the second argument to an integer
    k = parse(Int, ARGS[2])
    rmcc = RME(q, k = k)
else
    rmcc = RME(q)
end

df = DataFrame(rmcc, :auto)
filename = "../output/matrix_original.csv"
CSV.write(filename, df, header=false)
