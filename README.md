# Voximplant PDS client

## Build

To build a Voximplant PDS client, make sure the following requirements are installed
* Golang 1.17
* `make` utils 

Then run `make compile` to build the client.
  
## Protocol initialization

1. After opening a bidirectional connection via the `PDS.Start` method, send a PDS agent initialization request: `message RequestMessage` of the `INIT` type).
2. Wait for a response to the initialization request: `message ServiceMessage` of the `INIT_RESPONSE` type. The response contains the `session_id`. You can save it to use in future initializations to use the accumulated statistics.
3. Wait for the request from the server: `message ServiceMessage` of the `GET_TASK` type.
4. After receiving the `GET_TASK` request, send the desired tasks as soon as possible.
   
## Nuances

1. If you send more tasks than the server requested, or if you send the task before getting the request, the connection will be closed.
2. If you receive any Voximplant-related errors (e.g. scenario start error), the connection may be closed. In this case, call the `PDS.Start` method again and repeat the initialization process again.
   
## PDS behavior in different situations

- if you the number of operators increases, the task distribution becomes quicker
- if the call duration increases, the task distribution becomes slower
- if the answer percentage improves, the task distribution becomes slower
- if the number of free operators increases, the task distribution becomes slightly quicker
  
### PDS behavior when changing working conditions

- if the answer percentage suddenly increases, the customer queue peaks at the start. After the queue normalizes, PDS will work according to the new statistics
- if the answer percentage suddenly decreases, the operator waiting time decreases. It takes about 2-3 minutes for PDS to adapt to the new statistics (depends on the number of operators and the call duration: the higher the call duration, the more time it takes to adapt to the new statistics)
- if the call duration increases, the customer queue peaks at the start
- if the call duration decreases, the operator waiting time increases
- if the number of operators increases, the dialing time decreases. If the number of operators highly increases, the operator waiting time can temporarily increase
- if the number of operators decreases, the customer queue increases. After the queue normalizes, PDS will work according to the new statistics
  
## VoxEngine scenario

* The scenario parameters are received in the following format:
  
```
{
    "users_data":{}, // custom json with a phone number, customer's name and etc 
    "task_uuid" : "string",
    "agent_uuid": "string",
    "queue_id": 1,
    "queue_name": "string",
    "callback_url":"https://pds.voximplant.com/v1/result"
}
```

* Parameters parsing:
  
```javascript
var data = JSON.parse(VoxEngine.customData());
Logger.write(JSON.stringify(data.users_data)); // custom data with customer's information

function sendResult(status) {
	var pd = {
		agent: data.agent_uuid,
		task_uuid: data.task_uuid,
		type: status
	};
	Logger.write('SEND RESULT ' + status)
	if (sended) return false
	Net.httpRequest(data.callback_url, function (e) {
		if (e.code == 200) {
			Logger.write("Connected successfully");
			Logger.write("code:  " + e.code);
			sended = true
		} else {
			Logger.write("Unable to connect");
		}
		if (status === 'FAIL') VoxEngine.terminate()
	}, {
		rawOutput: true,
		method: "POST",
		postData: JSON.stringify(pd)
	});
}
```

* Send the following request if the call succeeds:
  
```javascript
sendResult("DIAL_COMPLETE");
```

* Send the following request if the call fails:
  
```javascript
sendResult("FAIL");
```