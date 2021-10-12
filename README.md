# local-message-server
A local messaging server i wrote for shits and giggles 

## Endpoints
`/message/all/`

`/message/get/{id}`

`/message/post/` Body {Contents: "", Address: "local ipv4 address"}

`/message/delete/{id}`
 
 
 `/user/set/?address={ipv4_address}&username={string}`
 
`/user/get/{address}`

`/events/` subs to the new message and message deleted events
