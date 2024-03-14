using Polynomials, Nemo, AbstractAlgebra, CSV, DataFrames, Serialization, DelimitedFiles
include("FastEvaluationOnPrimeField.jl")
################################################################################
#                 Examples
################################################################################
function randPoly(q, d)
    Fq, _ = FiniteField(q, 1, "x")
    R, _ = PolynomialRing(Fq, "x")
    f = R(rand(0:q-1, d))
    return f
end

function RMEncoding(message, q, d)
    Fq = FiniteField(q, 1, "x")[1]
    R, x = PolynomialRing(Fq, "x")

    matt = zeros(Int16, q, q)
    index = 1

    for i = 1:d+1
        matt[i, 1:d+2-i] = message[index: index + d+1-i]
        index += d+2-i
    end

    for j in d+1:-1:1
        x_values = [Fq(i) for i in 0:j-1]
        y_values = [Fq(i) for i in [matt[i, d+2-j] for i in 1:j]]

        for t in 0:d-j
            y_values -= [Fq(i)^(d-t) for i in 0:j-1] .* matt[d+1-t, d+2-j]
        end

        interp_poly = interpolate(R, x_values, y_values)

        for i in 0:j-1
            matt[i+1, d+2-j] = toInt(coeff(interp_poly, i))
        end
        
        x_values = [Fq(i) for i in 0:d+1-j]
        y_values = [Fq(i) for i in [matt[j, i] for i in 1:d+2-j]]
        interp_poly = interpolate(R, x_values, y_values)
        for k in d+3-j:q
            matt[j, k] = toInt(interp_poly(k-1))
        end
    end

    modtree = buildModTree(q)
    
    for col in 1:q
        matt[:, col] = [toInt(i) for i in uniVWithTree(R(matt[:, col]), q, modtree)]
    end

    return matt
end

function systematic_test(message, rmc, d)
    index = 0
    row = 1

    for i = d+1:-1:1
        @assert message[index+1:index+i] == rmc[row, 1:i]
        index += i
        row += 1
    end
end

function file_systematic_test(message, matrix_read, d)
    index = 0
    col = 1

    for i = d+1:-1:1
        @assert message[index+1:index+i] == matrix_read[1:i, col]
        index += i
        col += 1
    end
end

function simpleTest(rmc, q)
    for i in 1:q
        @assert sum(rmc[i,:]) % q == 0
        @assert sum(rmc[:,i]) % q == 0
    end
    @assert sum([rmc[i,i] for i in 1:q]) % q == 0
end

function RME(q::Int; k = 4) 
    d = (q-2)÷k
    message = rand(0:q-1, Int64((d+1)*(d+2)/2))
    rmcc = RMEncoding(message, q, d)
    systematic_test(message, rmcc, d)
    simpleTest(rmcc, q)
    writedlm(stdout, rmcc, ',')
end


q = parse(Int, ARGS[1])

if length(ARGS) >= 2
    # Parse the second argument to an integer
    k = parse(Int, ARGS[2])
    RME(q, k = k)
else
    RME(q)
end
# d = q÷4
# message = rand(0:q-1, Int64((d+1)*(d+2)/2))
# rmcc = RMEncoding(message, q, d)
# systematic_test(message, rmcc, d)
# simpleTest(rmcc, q)
# writedlm(stdout, rmcc, ',')

# # q = 2^10 15s
# # q = 2^11 96s
# # q = 2^12 629.597368
# # 629.597368 seconds (5.64 G allocations: 254.522 GiB, 21.04% gc time, 0.01% compilation time)

# # q = 2^13 simply for coeff.
# # 4666.636293 seconds (33.46 G allocations: 1.293 TiB, 39.18% gc time)
# # q = 2^12.5
# # 1781.417077 seconds (14.88 G allocations: 656.562 GiB, 25.73% gc time), 2153702696
# # q = 2^13 
# # 7391.344118 seconds (47.50 G allocations: 1.887 TiB, 20.31% gc time) 8209×8209 Matrix{Int16} 128M
# # df_read = CSV.read("matrix.csv", DataFrame)
# # matrix_read = Matrix{Int}(df_read)
# # file_systematic_test(message, matrix_read, d)


