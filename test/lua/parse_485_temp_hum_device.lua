---@diagnostic disable: undefined-global
-- {
--     "d1":{
--         "tag":"d1",
--         "weight":0,
--         "initValue":0,
--         "function":3,
--         "slaverId":1,
--         "address":0,
--         "quantity":2,
--         "value":"0194011b"
--     },
--     "d2":{
--         "tag":"d2",
--         "weight":0,
--         "initValue":0,
--         "function":3,
--         "slaverId":2,
--         "address":0,
--         "quantity":2,
--         "value":"018e0118"
--     }
-- }
--
---
--[[
{
  "method":"report",
  "clientToken":"rulex",
  "timestamp":1677762028638,
  "params":{
    "tag":1,
    "temp":1,
    "hum":32
  }
}
--]]
--

-- Actions
Actions =
{
    function(data)
        local dataT, err = rulexlib:J2T(data)
        if (err ~= nil) then
            print('parse json error:', err)
            return true, data
        end
        for key, value in pairs(dataT) do
            --
            -- local MatchHexS = rulexlib:MatchUInt64("temp:[0,1];hum:[2,3]", value['value'])
            --
            local MatchHexS = rulexlib:MatchHex("temp:[0,1];hum:[2,3]", value['value'])
            local ts = rulexlib:TsUnixNano()
            local Json = rulexlib:T2J(
                {
                    method = 'report',
                    requestId = ts,
                    timestamp = ts,
                    params = {
                        tag = key,
                        temp = MatchHexS['temp'],
                        hum = MatchHexS['hum'],
                    }
                }
            )
            print("DataToMqtt->OUT48320dfdeaaa4ec7971a37a922e17d93:", Json)
            rulexlib:DataToMqtt('OUT48320dfdeaaa4ec7971a37a922e17d93', Json)
        end
        return true, data
    end
}
