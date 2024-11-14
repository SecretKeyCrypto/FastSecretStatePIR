include("FFT.jl")
include("FastEvaluationOnPrimeField.jl")
using Nemo

function RME2DFFT(message, q, d)
    @assert (d+1)*(d+2)÷2 == length(message)
    @assert is_power_of_2(q-1)
    @assert is_power_of_2(d+1)

    Fq = finite_field(q, 1, "x")[1]
    R, x = polynomial_ring(Fq, "x")

    matt = zeros(Int64, q, q)
    index = 1

    F = GF(q)
    k = (q-1)÷(d+1)
    ω = find_primitive_root(F, q)
    # ω_inter is (d+1)-th root of unity, not d
    ω_inter = ω^k
    roots_of_unity_vector = [toInt(ω_inter^i) for i in 0:d]

    # Put the message on the point of (ω_inter ^ i, ω_inter ^ j, where i + j <= d+2)
    index = 1
    for i in 1:d+1
        for j in 1:d+2-i
            matt[roots_of_unity_vector[i], roots_of_unity_vector[j]] = message[index]
            index += 1
        end
    end

     # View Bi-variate polynomial P as
    #         P = f_0 * x^0 + f_1 * x +...+ f_d * x^d
    # while f_i is a univariage polynomail for the second variable.
    # for j in reverse(roots_of_unity_vector)
    for j in d+1:-1:1
        x_values = [Fq(i) for i in roots_of_unity_vector[1:j]]
        y_values = [Fq(i) for i in [matt[roots_of_unity_vector[i], roots_of_unity_vector[d+2-j]] for i in 1:j]]

        for t in 0:d-j
            y_values -= [Fq(i)^(d-t) for i in roots_of_unity_vector[1:j]] .* matt[roots_of_unity_vector[d+1-t], roots_of_unity_vector[d+2-j]]
        end

        # Determine the values of of f_(d+1-j)(0),...,f_(d+1-j)(d-j)
        interp_poly = interpolate(R, x_values, y_values)

        for i in 0:j-1
            matt[roots_of_unity_vector[i+1], roots_of_unity_vector[d+2-j]] = toInt(coeff(interp_poly, i))
        end
        
        x_values = [Fq(roots_of_unity_vector[i+1]) for i in 0:d+1-j]
        y_values = [Fq(i) for i in [matt[roots_of_unity_vector[j], roots_of_unity_vector[i]] for i in 1:d+2-j]]

        # Interpolate the coefficient of f_(d-j)
        interp_poly = interpolate(R, x_values, y_values)

        eval_poly = fft!(F, [F(toInt(coeff(interp_poly, i))) for i in 0:d+1-j], ω, q-1)

        matt[roots_of_unity_vector[j], 1] = toInt(coeff(interp_poly, 0))
        for k in 0:q-2
            matt[roots_of_unity_vector[j], toInt(ω^k)+1] = toInt(eval_poly[k+1])
        end
    end

    Threads.@threads for col in 1:q
        evaluate_at_roots_of_unity = fft!(F, [F(i) for i in [matt[roots_of_unity_vector[i], col] for i in 1:d+1]], ω, q-1)
        matt[1, col] = matt[roots_of_unity_vector[1], col]
        for t in 0:q-2
            matt[toInt(ω^t)+1, col] = toInt(evaluate_at_roots_of_unity[t+1])
        end
    end
    return matt
end

# q = 65537
# d = 4095
# message = [rand(0:q-1) for _ in 1:(d+1)*(d+2)÷2]
# @time RME2DFFT(message, q, d)

