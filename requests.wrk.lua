-- wrk2 --rate 100 --connections 10 --duration 15 --threads 2 --script requests.wrk.lua http://127.0.0.1:65176
local urand = assert (io.open ('/dev/urandom', 'rb'))

-- https://stackoverflow.com/questions/46236165/wrk-executing-lua-script
function RNG (b, m)
  b = b or 4
  m = m or 256
  local n, s = 0, urand:read (b)

  for i = 1, s:len () do
    n = m * n + s:byte (i)
  end

  return n
end

request = function()
   local r = math.fmod(RNG(1), 100)
   local path = "/data/" .. r
   wrk.headers["X-Request-Id"] = r
   return wrk.format(nil, path)
end
