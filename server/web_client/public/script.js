const PORT    = ":80";
const ADDRESS = "http://192.168.1.35"+PORT;

const eventSource = new EventSource(ADDRESS+'/events/');

eventSource.onmessage = (e) => {
    console.log('got message!');

    let ul = document.getElementById('messages');
    let li = document.createElement('li');

    let data = JSON.parse(e.data);

    li.appendChild(document.createTextNode(data.Content));
    ul.appendChild(li);
}

function set_li(content) {
    let ul = document.getElementById('messages');
    let li = document.createElement('li');

    li.appendChild(document.createTextNode(content));
    ul.appendChild(li);
}

(async () => {
    const data = await fetch(ADDRESS+'/message/all/');
    const messages = await data.json();

    for(const m of messages) {
        const user = await (await fetch(ADDRESS+'/user/get/'+m.UserIP)).text();
        set_li(user +': '+m.Content);
    }
})();

const form = document.getElementById('msg_form');
form.onsubmit = async (event) => {

    event.preventDefault();

    const contents = event.target.elements.message_content.value;
    const IP = await get_ip();

    console.log(IP);

    const res = await fetch(ADDRESS+'/message/post/', {
        body: JSON.stringify({
            Content: contents,
            Address: IP
        }),
        method: 'POST'
    });

    event.target.elements.message_content.value = "";

    set_li(contents);
}

async function get_ip() {
    await new Promise((resolve, reject) => {
        const conn = new RTCPeerConnection()
        conn.createDataChannel('')
        conn.createOffer(offer => conn.setLocalDescription(offer), reject)
        conn.onicecandidate = ice => {
          if (ice && ice.candidate && ice.candidate.candidate) {
            resolve(i.candidate.candidate.split(' ')[4])
            conn.close()
          }
        }
    })
}
