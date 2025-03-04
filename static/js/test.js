const accLink = document.getElementById('account').onclick = function () {
  html = `
    <h3>Update Account</h3>
    <label>Name:</label>
    <input type="text" placeholder="Enter Name"><br><br>
    <label>Email:</label>
    <input type="email" placeholder="Enter Email"><br><br>
    <button>Save</button>
  `;
}

function closePanel() {
  document.getElementById("sidePanel").style.display = "none";
}
