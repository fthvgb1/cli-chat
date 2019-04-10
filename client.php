<?php

$socket = socket_create(AF_INET,SOCK_STREAM,SOL_TCP);
$x = socket_connect($socket,'127.0.0.1',9988);
$host = '127.0.0.1';
$port = 9988;
$key = generateKey();
$headers = [
    'host' => $host . ":" . $port,
    'user-agent' => 'websocket-client-php',
    'connection' => 'Upgrade',
    'upgrade' => 'websocket',
    'sec-websocket-key' => $key,
    'sec-websocket-version' => '13',
];
$header =
    "GET " . '/' . " HTTP/1.1\r\n"
    . implode(
        "\r\n", array_map(
            function ($key, $value) {
                return "$key: $value";
            }, array_keys($headers), $headers
        )
    )
    . "\r\n\r\n";

$x = socket_write($socket,$header,strlen($header));
$response = socket_read($socket,1024);

preg_match('@Sec-WebSocket-Accept: ([\w=]+)@i',$response,$matches);
$keyAccept = trim($matches[1]);
$expectedResonse = base64_encode(pack('H*', sha1($key . '258EAFA5-E914-47DA-95CA-C5AB0DC85B11')));



$payload = 'hello sockets';
$pid1 = pcntl_fork();
if($pid1==0){
    while (1){
        $ll = fgets(STDIN);
        $payload=$ll;
        send($payload,$socket);
    }

}elseif($pid1==-1){
    echo 'error';exit;
}else{
    $pid2 = pcntl_fork();
    if($pid2==0){
        receive($socket);
    }
    pcntl_wait($status);
}

if($pid2==0){

}elseif($pid2==-1){
    echo 'error2';exit;
}else{
    pcntl_wait($x);
}

/**
 * @param $message
 * @param $socket resource
 */
function send($message,$socket){
    $payload = $message;
    $final = true;

    $frame_head_binstr = '';

// Write FIN, final fragment bit.
    $frame_head_binstr .= (bool) $final ? '1' : '0';

// RSV 1, 2, & 3 false and unused.
    $frame_head_binstr .= '000';

// Opcode rest of the byte.
    $frame_head_binstr .= sprintf('%04b', 1);

    $masked = true;
// Use masking?
    $frame_head_binstr .= $masked ? '1' : '0';

// 7 bits of payload length...
    $payload_length = strlen($payload);
    if ($payload_length > 65535) {
        $frame_head_binstr .= decbin(127);
        $frame_head_binstr .= sprintf('%064b', $payload_length);
    }
    elseif ($payload_length > 125) {
        $frame_head_binstr .= decbin(126);
        $frame_head_binstr .= sprintf('%016b', $payload_length);
    }
    else {
        $frame_head_binstr .= sprintf('%07b', $payload_length);
    }

    $frame = '';

// Write frame head to frame.
    foreach (str_split($frame_head_binstr, 8) as $binstr) {
        $frame .= chr(bindec($binstr));
    }

// Handle masking
    if ($masked) {
        // generate a random mask:
        $mask = '';
        for ($i = 0; $i < 4; $i++) $mask .= chr(rand(0, 255));
        $frame .= $mask;
    }

// Append payload to frame:
    for ($i = 0; $i < $payload_length; $i++) {
        $frame .= ($masked === true) ? $payload[$i] ^ $mask[$i % 4] : $payload[$i];
    }



    $w = socket_write($socket,$frame,strlen($frame));

}


function receive($socket)
{
    $me = '';
    while (1){
        $x = $data = socket_read($socket,2);
        $mask = (boolean)(ord($data[1]) >> 7);  // Bit 0 in byte 1
        $payload = '';

        // Payload length
        $payload_length = (integer)ord($data[1]) & 127;
        if ($payload_length > 125) {
            if ($payload_length === 126) {
                // 126: Payload is a 16-bit unsigned int
                $data = socket_read($socket,2);
            } else {
                // 127: Payload is a 64-bit unsigned int
                $data = socket_read($socket,8);
            }
            $length = strlen($data);

            for($i=0;$i<$length;++$i){
                $payload .=  sprintf("%08b", ord($data[$i]));
            }
            $payload_length = bindec($payload);
        }
        $mask_key = '';
        if($mask){
            $mask_key = socket_read($socket,4);
        }
        if($payload_length>0){
            $data = socket_read($socket,$payload_length);
            if($mask){
                for ($i = 0; $i < $payload_length; $i++) {
                    $payload .= ($data[$i] ^ $mask_key[$i % 4]);
                }
            }else{
                $payload = $data;
            }
        }
        $tmp = json_decode($payload,true);
        if(!isset($tmp['content'])){
            $me = $tmp['sender'];
            $tmp['content'] = '';
        }
        echo  ($me==$tmp['sender'] ?'我':$tmp['sender']),':',$tmp['content'],"\n\r";



    }

}



 function generateKey()
{
    $chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!"$&/()=[]{}0123456789';
    $key = '';
    $chars_length = strlen($chars);
    for ($i = 0; $i < 16; $i++) $key .= $chars[mt_rand(0, $chars_length - 1)];
    return base64_encode($key);
}