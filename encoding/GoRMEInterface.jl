include("../encoding/RMEncoding.jl")
using CSV, DataFrames

for line in eachline(stdin)
    input, output, arg_q, arg_k = split(line, " ")
    q = parse(Int, arg_q)
    k = parse(Int, arg_k)

    df = CSV.read(input, DataFrame, header=false)

    message = collect(Tuple(df[1, :]))

    n = length(message)

    d = Int(ceil((-3 + sqrt(1 + 8*n)) / 2))

    if (d * k + 1 >= q)
        throw(ErrorException("The length of n ($n) message can't be encoded with field size ($q) and degree ($k)."))
    elseif (n < (d+1) * (d+2) ÷ 2)
        append!(message, rand(0:q-1, Int64((d+1)*(d+2)/2 - n)))
    end

    rmc = RME(message, d, q)
    df = DataFrame(rmc, :auto)
    CSV.write(output, df, header=false)
    println()
end
