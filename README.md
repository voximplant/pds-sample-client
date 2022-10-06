## Build
### Requirements
* Golang 1.17
* `make` utils

Run `make compile` command to build the app.

## Protocol usage
1. A `PDS.Start` method is used to open a bidirectional connection. Right after the method execution, you have to send an initialization request for a PDS agent (`message RequestMessage`, request type: `INIT`).
2. Then wait for `message ServiceMessage` of type `INIT_RESPONSE`. This response contains a `session_id` field which value has to be stored and used in further initializations to have access to accumulated stats.
3. Wait for request from a server, `message ServiceMessage` of type `GET_TASK`. 
4. After receiving a `GET_TASK` request you have to send the needed volume of tasks as quick as possible.

## Features
1. If you send more tasks than needed or send tasks without receiving an appropriate request, the connection will be closed.
2. Any Voximplant-related errors (e.g., an error of scenario starting) can cause the connection close. In such a case, you have to call the `PDS.Start` method again and follow the initialization procedure to open a bidirectional connection.

## Distribution Changes
– increasing of the number of operators speeds up distribution of tasks
– increasing of conversation and processing time slows down distribution of tasks
– increasing of hit rate slows down distribution of tasks
– number of free operators speeds up distribution of tasks (not significantly)
 

### Possible Circumstances 
– hit rate has increased drastically: there will be a peak queue of incoming customers. When the queue size reduces, PDS will work according to a new hit rate value.
– hit rate has decreased drastically: the waiting time increases and it takes about 2-3 minutes for processing to adjust to new conditions and become stable again. The final adjusting time depends on the number of operators and especially on conversation and processing time – the bigger the latter is, the longer the adjusting time.
– conversation time has increased: there will be a peak queue of incoming customers.
– conversation time has decreased: the waiting time will be longer.
- the number of operators has increased: in general, calling to customers will be increased too, but if the number of operators has increased **drastically**, the waiting time could be temporarily longer. 
– the number of operators has decreased: there will be a peak queue of incoming customers. When the queue size reduces, PDS will work according to current conditions.

## VoxEngine Scenario
* Parameters are received in the following format:
```
{
    "users_data":{}, // custom json with a phone number, customer's name, etc. 
    "task_uuid" : "string",
    "agent_uuid": "string",
    "queue_id": 1,
    "queue_name": "string",
    "callback_url":"https://pds.voximplant.com/v1/result"
}
```
* Parameters' parcing:
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
* If a call is successful, a request should be sent:
```javascript
sendResult("DIAL_COMPLETE");
```
* If a call is failed, another request should be sent:
```javascript
sendResult("FAIL");
```