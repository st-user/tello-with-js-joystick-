/* global xyJoyStickUI, zrJoyStickUI */

const $connect = document.querySelector('#connect');
const $takeoff = document.querySelector('#takeoff');
const $land = document.querySelector('#land');
const $videoOn = document.querySelector('#videoOn');
const $videoOff = document.querySelector('#videoOff');
const $disconnect = document.querySelector('#disconnect');

const $mainSection = document.querySelector('#mainSection');
const $videoEmpty = document.querySelector('#videoEmpty');
const $videoContainer = document.querySelector('#videoContainer');
const $video = document.querySelector('#video');

const STATE = {
    INIT: 0,
    CONNECTED: 1,
    VIDEO_ON: 2,
    DISCONNECTED: 3
};

let state = STATE.INIT;
let rtcPeerConnection;
let xyCoordToSend = undefined;
let zrCoordToSend = undefined;

$connect.addEventListener('click', async event => {
    event.preventDefault();
    if (state !== STATE.INIT || state === STATE.DISCONNECTED) {
        return;
    }
    state = STATE.CONNECTED;
    changeButtonStateOnConnect();
    await simpleAccess('/connect');
});

$disconnect.addEventListener('click', async event => {
    event.preventDefault();
    if (state === STATE.INIT || state === STATE.DISCONNECTED) {
        return;
    }

    let msg = 'Server application process is killed and stop when \'yes/ok\' is selected.';
    msg += 'If you want to connect the application again, ';
    msg += 'please start the server application manually and reload this page.';

    if (!confirm(msg)) {
        return;
    }
    state = STATE.DISCONNECTED;
    changeButtonStateOnDisconnect();
    await simpleAccess('/disconnect');
});

$takeoff.addEventListener('click', async event => {
    event.preventDefault();
    if (state === STATE.INIT || state === STATE.DISCONNECTED) {
        return;
    }
    await simpleAccess('/takeoff');
});

$land.addEventListener('click', async event => {
    event.preventDefault();
    if (state === STATE.INIT || state === STATE.DISCONNECTED) {
        return;
    }
    await simpleAccess('/land');
});

$videoOn.addEventListener('click', async event => {
    event.preventDefault();
    if (state !== STATE.CONNECTED || state === STATE.DISCONNECTED) {
        return;
    }
    state = STATE.VIDEO_ON;
    disableElement($videoOn);
    enableElement($videoOff);
    await negotiate();
});

$videoOff.addEventListener('click', async event => {
    event.preventDefault();
    if (state !== STATE.VIDEO_ON || state === STATE.DISCONNECTED) {
        return;
    }
    state = STATE.CONNECTED;
    enableElement($videoOn);
    disableElement($videoOff);
    closeRTCConnection();
    await simpleAccess('/videoOff');
});

window.addEventListener('resize', resizeVideo);

/*
 * Events triggered by controlling the joysticks defined in 'joystick.js'
 */
xyJoyStickUI.onmove(data => {
    const coords = data.coords;
    xyCoordToSend = {
        x: coords.inUI.x / xyJoyStickUI.radius,
        y: coords.inUI.y / xyJoyStickUI.radius
    }; 
});
xyJoyStickUI.onend(() => {
    xyCoordToSend = undefined;
    sendJoystickXy({ x: 0, y: 0 });
});

zrJoyStickUI.onmove(data => {
    const coords = data.coords;
    zrCoordToSend = {
        z: coords.inUI.y / zrJoyStickUI.radius,
        r: coords.inUI.x / zrJoyStickUI.radius
    };
});
zrJoyStickUI.onend(() => {
    zrCoordToSend = undefined;
    sendJoystickZr({ z: 0, r: 0 });
});

/*
 * Initialize contents.
 */ 
display($videoEmpty, true);
display($videoContainer, false);
initButtonState();
resizeVideo();
setTimeout(doSendJoystickXy, 100);
setTimeout(doSendJoystickZr, 100);


async function simpleAccess(url) {
    await fetch(url)
        .then(res => {
            if (res.status !== 200) {
                console.error('error', res.statusText);
            }
        });
}

function postJson(url, obj) {
    return fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(obj)
    }).then(res => {
        if (res.status !== 200) {
            console.error('error', res.statusText);
            return {};
        }
        const len = parseInt(res.headers.get('content-length'));
        if (len === 0) {
            return {};
        }
        return res.json();
    });
}

function display($elem, isBlock) {
    $elem.style.display = isBlock ? 'block' : 'none';
}

function initButtonState() {
    enableElement($connect);
    disableElement($takeoff);
    disableElement($land);
    disableElement($videoOn);
    disableElement($videoOff);
    disableElement($disconnect);
}

function changeButtonStateOnConnect() {
    disableElement($connect);
    enableElement($takeoff);
    enableElement($land);
    enableElement($videoOn);
    disableElement($videoOff);
    enableElement($disconnect);
}

function changeButtonStateOnDisconnect() {
    disableElement($connect);
    disableElement($takeoff);
    disableElement($land);
    disableElement($videoOn);
    disableElement($videoOff);
    disableElement($disconnect);
}

function disableElement($elem) {
    resetClass($elem, 'enabled', 'disabled');
}

function enableElement($elem) {
    resetClass($elem, 'disabled', 'enabled');
}

function resetClass($elem, classToRemove, classToAdd) {
    $elem.classList.remove(classToRemove);
    $elem.classList.add(classToAdd);
}

/*
 * WebRTC
 */
async function negotiate() {
    createPeerConnection();
    const gather = () => {
        return new Promise(function(resolve) {
            console.debug('gather', rtcPeerConnection.iceGatheringState);
            if (rtcPeerConnection.iceGatheringState === 'complete') {
                resolve();
            } else {
                const checkState = () => {
                    console.debug('gather', rtcPeerConnection.iceGatheringState);
                    if (rtcPeerConnection.iceGatheringState === 'complete') {
                        rtcPeerConnection.removeEventListener('icegatheringstatechange', checkState);
                        resolve();
                    }
                };
                rtcPeerConnection.addEventListener('icegatheringstatechange', checkState);
            }
        });
    };

    const transceiver = rtcPeerConnection.addTransceiver('video');
    transceiver.direction = 'recvonly';
    const offer = await rtcPeerConnection.createOffer();
    await rtcPeerConnection.setLocalDescription(offer);
    await gather();
    // Use Vanilla ICE for simplicity.
    const offerLocalDesc = rtcPeerConnection.localDescription;

    const answer = await postJson('offer', {
        sdp: offerLocalDesc.sdp,
        type: offerLocalDesc.type,
    });
    console.log(answer);
    rtcPeerConnection.setRemoteDescription(answer);
}

function createPeerConnection() {

    const config = {};
    rtcPeerConnection = new RTCPeerConnection(config);

    function addEventListenerShowingState(eventName, propName) {
        rtcPeerConnection.addEventListener(eventName, () => {
            console.debug(`${eventName}: ${rtcPeerConnection[propName]}`);
        });        
    }
    addEventListenerShowingState('connectionstatechange', 'iceGatheringState');
    addEventListenerShowingState('icegatheringstatechange', 'iceGatheringState');
    addEventListenerShowingState('iceconnectionstatechange', 'iceConnectionState');
    addEventListenerShowingState('signalingstatechange', 'signalingState');

    rtcPeerConnection.addEventListener('track', async event => {
        console.debug('track', event.streams);
        console.debug('kind', event.track.kind);
        if (event.track.kind === 'video') {
            display($videoEmpty, false);
            display($videoContainer, true);
            $video.onloadedmetadata = () => {
                resizeVideo();
                console.debug('Video metadata loaded.')
            };
            $video.srcObject = event.streams[0];
        }          
    });
}

function closeRTCConnection() {

    if (rtcPeerConnection) {
        if (rtcPeerConnection.getTransceivers) {
            rtcPeerConnection.getTransceivers().forEach(transceiver => {
                if (transceiver.stop) {
                    transceiver.stop();
                }
            });
        }
    
        rtcPeerConnection.getSenders().forEach(sender => {
            if (sender.track && sender.track.stop) {
                sender.track.stop();
            }
        });
    
        rtcPeerConnection.close();
    }
}

function resizeVideo() {
    const videoWidth = $video.videoWidth;
    const videoHeight = $video.videoHeight;
    const aspectRatio = !videoHeight ? 16/9 : videoWidth / videoHeight; 

    let videoSectionWidth = window.innerWidth;
    let videoSectionHeight = videoSectionWidth / aspectRatio;
    const windowHeight = window.innerHeight * 0.9;
    if (windowHeight < videoSectionHeight) {
        videoSectionHeight = windowHeight;
        videoSectionWidth = videoSectionHeight * aspectRatio;
    }
    $mainSection.style.width = `${videoSectionWidth}px`;
    $mainSection.style.height = `${videoSectionHeight}px`;
    $videoEmpty.style.width = `${videoSectionWidth}px`;
    $videoEmpty.style.height = `${videoSectionHeight}px`;
    $video.style.width = `${videoSectionWidth}px`;
    $video.style.height = `${videoSectionHeight}px`;
}

/*
 * Sends a x,y coordinate each 100ms
 */
async function doSendJoystickXy() {
    await sendJoystickXyIfNecessary();
    setTimeout(doSendJoystickXy, 100);
}

async function sendJoystickXyIfNecessary() {
    if (!xyCoordToSend) {
        return;
    }
    await sendJoystickXy(xyCoordToSend);
}

function sendJoystickXy(xy) {
    return postJson('/moveXy', xy);
}

/*
 * Sends a z,r coordinate each 100ms
 */
async function doSendJoystickZr() {
    await sendJoystickZrIfNecessary();
    setTimeout(doSendJoystickZr, 100);
}

async function sendJoystickZrIfNecessary() {
    if (!zrCoordToSend) {
        return;
    }
    await sendJoystickZr(zrCoordToSend);
}

function sendJoystickZr(zr) {
    return postJson('/moveZr', zr);
}