let canvas = document.getElementById("board");
let ctx = canvas.getContext("2d");
const gooner = {
    x: 20,
    y: canvas.height - 50,
    velocity: 0,
    gravity: 0.4,
    ground: false,
    ladder: false,
    thickness: 20,
    height: 30,
    speed: 2
};
const floor = {
    x: 0,
    y: canvas.height-20,
    thickness: 800,
    height: 20
};
const floor2 = {
    x: 0,
    y: canvas.height-120,
    thickness: 720,
    height: 20
};
const floor3 = {
    x: 60,
    y: canvas.height-220,
    thickness: 740,
    height: 20
};
const floor4 = {
    x: 0,
    y: canvas.height-320,
    thickness: 720,
    height: 20
};
const floor5 = {
    x: 60,
    y: canvas.height-420,
    thickness: 740,
    height: 20
};
const floor6 = {
    x: 0,
    y: canvas.height-520,
    thickness: 720,
    height: 20
};
const floor7 = {
    x: 300,
    y: canvas.height-620,
    thickness: 160,
    height: 20
};
const ladder = {
    x: 720,
    y: canvas.height-120,
    height: 100,
    thickness: 20
};
const ladder2 = {
    x: 60,
    y: canvas.height-220,
    thickness: 20,
    height: 100
};
const ladder3 = {
    x: 720,
    y: canvas.height-320,
    thickness: 20,
    height: 100
};
const ladder4 = {
    x: 60,
    y: canvas.height-420,
    thickness: 20,
    height: 100
};
const ladder5 = {
    x: 720,
    y: canvas.height-520,
    thickness: 20,
    height: 100
};
const ladder6 = {
    x: 260,
    y: canvas.height-620,
    thickness: 40,
    height: 100
};

function draw() {
    ctx.clearRect(0, 0, canvas.width, canvas.height, );
    ctx.fillStyle = "red";
    ctx.fillRect(gooner.x, gooner.y, gooner.thickness, gooner.height);
    ctx.fillStyle = "brown";
    ctx.fillRect(floor.x, floor.y, floor.thickness, floor.height);
    ctx.fillStyle = "hotpink";
    ctx.fillRect(floor2.x, floor2.y, floor2.thickness, floor2.height);
    ctx.fillRect(floor3.x, floor3.y, floor3.thickness, floor3.height);
    ctx.fillRect(floor4.x, floor4.y, floor4.thickness, floor4.height);
    ctx.fillRect(floor5.x, floor5.y, floor5.thickness, floor5.height);
    ctx.fillRect(floor6.x, floor6.y, floor6.thickness, floor6.height);
    ctx.fillRect(floor7.x, floor7.y, floor7.thickness, floor7.height);
    ctx.fillStyle = "cyan"
    ctx.fillRect(ladder.x, ladder.y, ladder.thickness, ladder.height);
    ctx.fillRect(ladder2.x, ladder2.y, ladder2.thickness, ladder2.height);
    ctx.fillRect(ladder3.x, ladder3.y, ladder3.thickness, ladder3.height);
    ctx.fillRect(ladder4.x, ladder4.y, ladder4.thickness, ladder4.height);
    ctx.fillRect(ladder5.x, ladder5.y, ladder5.thickness, ladder5.height);
    ctx.fillRect(ladder6.x, ladder6.y, ladder6.thickness, ladder6.height);

    ctx.fillStyle = "red";
    ctx.fillRect(gooner.x, gooner.y, gooner.thickness, gooner.height);
}

draw();

function checkLadderCollision(ladder) {
    const ladderLeft = ladder.x;
    const ladderRight = ladder.x + ladder.thickness;

    const ladderTop = ladder.y + ladder.height;
    const ladderBottom = ladder.y;

    const goonerLeft = gooner.x;
    const goonerRight = gooner.x + gooner.thickness;
    const goonerTop = gooner.y + gooner.height;
    const goonerBottom = gooner.y;

    const isOutside =
        goonerRight < ladderLeft ||
        goonerLeft > ladderRight ||
        goonerBottom > ladderTop ||
        goonerTop < ladderBottom;

    return !isOutside;
}
function checkFloorCollision(floor) {
    const goonerBottom = gooner.y + gooner.height;
    const goonerRight = gooner.x + gooner.thickness;

    const touchingTop =
        goonerBottom >= floor.y &&
        goonerBottom <= floor.y + 10;

    const insideX =
        goonerRight > floor.x &&
        gooner.x < floor.x + floor.thickness;

    if (touchingTop && insideX && gooner.velocity >= 0) {
        gooner.y = floor.y - gooner.height;
        gooner.velocity = 0;
        return true;
    }
    else {
        return false;
    }
}

const keys={};
document.addEventListener("keydown", e=>keys[e.key] = true);
document.addEventListener("keyup", e=>keys[e.key] = false);
const gravity = 0;
const floorY = 0;
function update() {
    if (keys["a"] && gooner.x > 0) gooner.x -= gooner.speed;
    if (keys["d"] && gooner.x < (800 - gooner.thickness)) gooner.x += gooner.speed;
    if (!gooner.ladder) {
    gooner.velocity += gooner.gravity;
    gooner.y += gooner.velocity;
    gooner.ground = false;
}
    gooner.ground =
    checkFloorCollision(floor) ||
    checkFloorCollision(floor2) ||
    checkFloorCollision(floor3) ||
    checkFloorCollision(floor4) ||
    checkFloorCollision(floor5) ||
    checkFloorCollision(floor6) ||
    checkFloorCollision(floor7);
    
    gooner.ladder =
    checkLadderCollision(ladder) ||
    checkLadderCollision(ladder2) ||
    checkLadderCollision(ladder3) ||
    checkLadderCollision(ladder4) ||
    checkLadderCollision(ladder5) ||
    checkLadderCollision(ladder6);
    if (gooner.ladder) {
        gooner.velocity = 0;
    }
    if (keys["l"] && gooner.ground && !gooner.ladder) {
        gooner.velocity = -6;
        gooner.ground = false;
    }
    if (keys["w"] && gooner.ladder) {
        gooner.y -= 2;
    }
    if (keys["s"] && gooner.ladder) {
        gooner.y += 2;
    }
}

const intervalId = setInterval(() => {
    update();
    draw();
}, 10);
