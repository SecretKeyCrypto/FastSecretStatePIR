using Polynomials, Nemo, AbstractAlgebra, CSV, DataFrames, Serialization, DelimitedFiles
include("FastEvaluationOnPrimeField.jl")
################################################################################
#                 Q-ary Systematic Reed-Muller Encoding RM(2, d)
# Input:
#    - message: A list of integers in [0,...,q-1]
#    - q: Finite Field Size
#    - d: Total Degree of the Polynomial for Reed-Muller Code
#    - m: Dimension of the RM code, only support value 2 now.
#    - h: Last h values of the message are planted as 0's
# Output:
#    - A RM(m, d) Code
################################################################################
function RMEncoding(message, q, d, m)
    Fq = finite_field(q, 1, "x")[1]
    R, x = polynomial_ring(Fq, "x")

    matt = zeros(Int64, q, q)
    index = 1

    # Put the message on the upper-left triange, which means the matt[i][j] where i+j <= d+2
    for i = 1:d+1
        matt[1:d+2-i, i] = message[index: index + d+1-i]
        index += d+2-i
    end

    # View Bi-variate polynomial P as
    #         P = f_0 * x^0 + f_1 * x +...+ f_d * x^d
    # while f_i is a univariage polynomail for the second variable.
    for j in d+1:-1:1
        x_values = [Fq(i) for i in 0:j-1]
        y_values = [Fq(i) for i in [matt[i, d+2-j] for i in 1:j]]

        for t in 0:d-j
            y_values -= [Fq(i)^(d-t) for i in 0:j-1] .* matt[d+1-t, d+2-j]
        end

        # Determine the values of of f_(d-j)(0),...,f_(d-j)(d-j)
        interp_poly = interpolate(R, x_values, y_values)

        for i in 0:j-1
            matt[i+1, d+2-j] = toInt(coeff(interp_poly, i))
        end
        
        x_values = [Fq(i) for i in 0:d+1-j]
        y_values = [Fq(i) for i in [matt[j, i] for i in 1:d+2-j]]

        # Interpolate the coefficient of f_(d-j)
        interp_poly = interpolate(R, x_values, y_values)
        for k in d+3-j:q
            matt[j, k] = toInt(interp_poly(k-1))
        end
    end

    modtree = buildModTree(q)

    # Now in each col, the first q elements are evaluations of f_j on 0,...,q-1,
    # Then view each col as a univariate polynomial w.r.t. the second variable.
    # This evaluation coincide with the bi-variable polynomial evaluation.
    for col in 1:q
        matt[:, col] = [toInt(i) for i in uniVWithTree(R(matt[:, col]), q, modtree)]
    end

    return matt
end

function systematic_test(message, rmc, d)
    index = 0
    col = 1

    for i = d+1:-1:1
        @assert message[index+1:index+i] == rmc[1:i, col]
        index += i
        col += 1
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

function RME(message::Vector{Int}, d::Int, q::Int, m::Int)
    @assert binomial(d+m, m) == length(message)
    rmc = RMEncoding(message, q, d, m)
    systematic_test(message, rmc, d)
    simpleTest(rmc, q)
    return rmc
end