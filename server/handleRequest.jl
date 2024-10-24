using HTTP
using JSON
using DataFrames, CSV

function read_matrix_from_csv(file_path::String)
    df = CSV.read(file_path, DataFrame, header=false)
    data_matrix = Matrix(df)  # Convert DataFrame to 2D matrix
    
    # Determine dimensions of the matrix
    num_rows, num_cols = size(data_matrix)

    # Check if it's q*q (square) or q*q*q (flattened 3D)
    if num_rows == num_cols
        # It's a q*q matrix, return it as is
        return data_matrix
    elseif num_rows % num_cols == 0
        # It's a q*q*q matrix (flattened), reshape into 3D
        q = num_cols
        return reshape(data_matrix, q, q, q)
    else
        error("The CSV does not correspond to a q*q or q*q*q matrix.")
    end
end

# Function to sum values at given single-index positions
function sum_positions_single_index(q, matrix, indices::Array{Int,1})
    total_sum = 0
    for idx in indices
        if idx > 0 && idx <= length(matrix)
            # Because of different indexing between Go and Julia
            total_sum += matrix[idx + 1]
            total_sum %= q
        else
            return -1  # Error indicator for invalid index
        end
    end
    return total_sum
end

function handle_request(req::HTTP.Request, A_sliced, q)
    start_time = time_ns()
    if req.method == "POST"
        try
            body = String(req.body)
            data = JSON.parse(body)

            results = []
            if all(x -> isa(x, Number), data)
                indices = map(Int, data)  # Convert to Int
                for A in A_sliced
                    result = sum_positions_single_index(q, A, indices)
                    push!(results, result)
                end
            else
                HTTP.Response(400, "Invalid Request")
            end

            end_time = time_ns()
            elapsed_time_seconds = (end_time - start_time) / 1e9
            println("Request handled in $elapsed_time_seconds seconds")

            # Return the list of results for each matrix
            return HTTP.Response(200, JSON.json(Dict("sums" => results)))

        catch e
            println("Error parsing JSON: ", e)
            return HTTP.Response(400, "Invalid JSON format")
        end
    else
        return HTTP.Response(405, "Method Not Allowed")
    end
end

function LoadDatabase(filenames::Vector{String})
    A_sliced = []  # List to store the matrices

    for filename in filenames
        A = read_matrix_from_csv(filename)  # Load matrix from the current CSV file        
        push!(A_sliced, A)
    end

    q = size(A_sliced[1])[1]

    return A_sliced, q  # Return list of matrices and list of sizes
end

# Main logic: Get filenames from the command line
function main()
    if length(ARGS) == 0
        println("Please provide CSV filenames as command-line arguments.")
        return
    end

    # Read filenames from the command-line arguments (ARGS)
    filenames = ARGS

    # Initialize matrices by loading them from the provided filenames
    A_sliced, q = LoadDatabase(filenames)

    # Start the HTTP server
    HTTP.serve("0.0.0.0", 8080) do req
        handle_request(req, A_sliced, q)
    end
end

main()