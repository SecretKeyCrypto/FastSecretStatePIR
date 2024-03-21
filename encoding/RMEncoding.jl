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

    matt = zeros(Int64, q, q)
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
    rmc = RMEncoding(message, q, d)
    systematic_test(message, rmc, d)
    simpleTest(rmc, q)
    return rmc
end