/*
 * The class provides the functionality for detecting the selected coordinate on a joystick area.
 */
class JoyStickUI {

    constructor(params) {

        const COLOR = {
            BG: 'rgba(66, 172, 203, 0.5)',
            ICON: 'rgba(204, 49, 74, 0.8)',
            CIRCLE: 'rgba(204, 49, 74, 0.3)'
        };

        const _canvasSelector = params.selector;
        const _radius = params.radius || 150;
        const _styles = params.styles || {};
        _styles.pointerRadius = _styles.pointerRadius || _radius * 0.4;
        _styles.bg = _styles.bg || COLOR.BG;
        _styles.icon = _styles.icon || COLOR.ICON;
        _styles.circle = _styles.circle || COLOR.CIRCLE;

        const $canvas = document.querySelector(_canvasSelector);
        $canvas.width = _radius * 2;
        $canvas.height = _radius * 2;
        const canvasCenter = { x: $canvas.width / 2, y: $canvas.height / 2 };
        let canvasOffset = getElementPosition($canvas);

        this.$canvas = $canvas;
        this.center = canvasCenter;
        this.radius = _radius;
        this.styles = _styles;
        this._isMousedown = false;
        this._onstartHandlers = [];
        this._onmoveHandlers = [];
        this._onendHandlers = [];

        const self = this;
        function start(event) {
            event.preventDefault();
            canvasOffset = getElementPosition($canvas);

            self._isMousedown = true;
            self._onstartHandlers.forEach(h => {
                h.call(self);
            });
        }

        function end(event) {
            event.preventDefault();

            if (!self._isMousedown) {
                return;
            }
            self._isMousedown = false;

            self.drawBase(false);
            self._onendHandlers.forEach(h => {
                h.call(self);
            });
        }

        function move(event) {
            event.preventDefault();

            if (!self._isMousedown) {
                return;
            }

            const px = event.pageX;
            const py = event.pageY;
            // console.debug(`${px} ${py}`);
            let xInCanvas = px - canvasOffset.left;
            let yInCanvas = py - canvasOffset.top;
            let coordX = (xInCanvas - canvasCenter.x);
            let coordY = (canvasCenter.y - yInCanvas);

            let distance = Math.sqrt(coordX*coordX + coordY*coordY);
            if (_radius < distance) {

                const _px = px - canvasCenter.x - canvasOffset.left;
                const _py = canvasCenter.y - py + canvasOffset.top;
                let _a, _x, _y;
                if (!_px) {
                    _x = 0;
                    _y = (_py / Math.abs(_py)) * _radius;
                } else {
                    _a = _py / _px;
                    _x = (_px / Math.abs(_px)) * _radius / Math.sqrt(_a * _a + 1);
                    _y = _a * _x;
                }
       
                coordX = _x;
                coordY = _y;
            }
        
            self.drawBase(true);
            self.drawPointer(coordX, coordY);

            self._onmoveHandlers.forEach(h => {
                h.call(self, { 
                    coords: {
                        inUI: { x: coordX, y: coordY }
                    }
                });
            });
        }
    
        $canvas.addEventListener('mousedown', event => {
            start(event);
            move(event);
        });       
        window.addEventListener('mouseup', end);
        window.addEventListener('mousemove', move);

        function getElementPosition(element) {
            /*
             * reference:
             * https://developer.mozilla.org/en-US/docs/Web/API/Element/getBoundingClientRect#value
             *
             */
            const rect = element.getBoundingClientRect();
            // console.debug(`${rect.left} ${rect.top} ${window.scrollX} ${window.scrollY}`);
            return {
                left: rect.left + window.scrollX,
                top: rect.top + window.scrollY
            };

        }
    }

    active() {
        return this._isMousedown;
    }

    onstart(handler) {
        this._onstartHandlers.push(handler);
        return this;
    }

    onmove(handler) {
        this._onmoveHandlers.push(handler);
        return this;
    }

    onend(handler) {
        this._onendHandlers.push(handler);
        return this;
    }

    /* methods for drawing */

    drawBase(omitPointer) {
        this._drawBase();

        if (!omitPointer) {
            this.drawPointer(0, 0);
        }
    }

    _drawBase() {
        const $canvas = this.$canvas;
        const center = this.center;
        const circleFillStyle = this.styles.bg;

        const ctx = $canvas.getContext('2d');
        const width = $canvas.width;
        const height = $canvas.height;
        const gradient = ctx.createRadialGradient(
            center.x, center.y, 0,
            center.x, center.y, width / 2,
        );
        gradient.addColorStop(0, circleFillStyle);
        gradient.addColorStop(0.9, circleFillStyle);
        gradient.addColorStop(1, 'white');
        ctx.fillStyle = gradient;
    
        ctx.clearRect(0, 0, width, height);
        ctx.lineWidth = 1;
    
        ctx.beginPath();
        ctx.arc(center.x, center.y, width / 2, 0, Math.PI*2);
        ctx.fill();
    }

    drawPointer(coordX, coordY) {
        const $canvas = this.$canvas;
        const center = this.center;
        const fillStyle = this.styles.circle;
        const maxDistance = this.radius;
        const radius = this.styles.pointerRadius;

        const ctx = $canvas.getContext('2d');
        ctx.fillStyle = fillStyle;
    
        let circleCenterCoordX = coordX;
        let circleCenterCoordY = coordY;
        const distance = Math.sqrt(coordX * coordX + coordY * coordY);
    
        if (maxDistance < distance + radius) {
            const distanceToCenter = maxDistance - radius;
            circleCenterCoordX = coordX * distanceToCenter / distance;
            circleCenterCoordY = coordY * distanceToCenter / distance;
        }
        const circleCenterX = center.x + circleCenterCoordX;
        const circleCenterY = center.y - circleCenterCoordY;
    
        ctx.beginPath();
        ctx.arc(circleCenterX, circleCenterY, radius, 0, Math.PI * 2);
        ctx.closePath();
        ctx.fill();
    }

    drawDirectionIcon(x, y, length, rotationRadius = 0) {
        const $canvas = this.$canvas;
        const strokeStyle = this.styles.icon;

        const ctx = $canvas.getContext('2d');
        ctx.strokeStyle = strokeStyle;
        ctx.lineWidth = 3;
        
        const p1 = this._rotate(-length, -length, rotationRadius);
        const p3 = this._rotate(-length, length, rotationRadius);
        ctx.beginPath();
        ctx.moveTo(x + p1.x, y + p1.y);
        ctx.lineTo(x, y);
        ctx.lineTo(x + p3.x, y + p3.y);
        ctx.stroke();
    }

    drawRotationIcon(x, y, radiusX, radiusY, direction, arrowLength) {
        const $canvas = this.$canvas;
        const fillStyle = this.styles.icon;

        const ctx = $canvas.getContext('2d');
        ctx.fillStyle = fillStyle;
        ctx.lineWidth = 3;
    
        ctx.beginPath();
        if (!direction) {
            ctx.ellipse(x, y, radiusX, radiusY, 0, 0, Math.PI * 2 * 3 / 4, false);
            ctx.moveTo(x - arrowLength, y - radiusY - arrowLength);
            ctx.lineTo(x, y - radiusY);
            ctx.lineTo(x - arrowLength, y - radiusY + arrowLength);
        } else {
            ctx.ellipse(x, y, radiusX, radiusY, 0, Math.PI, Math.PI * 2 * 3 / 4, true);
            ctx.moveTo(x + arrowLength, y - radiusY - arrowLength);
            ctx.lineTo(x, y - radiusY);
            ctx.lineTo(x + arrowLength, y - radiusY + arrowLength);
        }
        ctx.stroke();   
    }

    _rotate(x, y, theta) {
        return {
            x: Math.cos(theta) * x - Math.sin(theta) * y,
            y: Math.sin(theta) * x + Math.cos(theta) * y
        };
    }
}










/* 
 * The joystick controlling Z coordinates and rotations(R).
 */
class ZrJoyStickUI extends JoyStickUI { // eslint-disable-line no-unused-vars

    constructor(params) {
        super(params);
    }

    drawBase(omitPointer) {
        super.drawBase(omitPointer);

        const iconDistanceFromCenter = this.radius * 0.75;
        const center = this.center;

        this.drawDirectionIcon(
            center.x, center.y - iconDistanceFromCenter - 10,
            10, -Math.PI / 2
        );
        this.drawDirectionIcon( 
            center.x, center.y - iconDistanceFromCenter + 10,
            10, -Math.PI / 2
        );
    
        this.drawDirectionIcon(
            center.x, center.y + iconDistanceFromCenter - 10,
            10, Math.PI / 2
        );
        this.drawDirectionIcon(
            center.x, center.y + iconDistanceFromCenter + 10,
            10, Math.PI / 2
        );
    
        this.drawRotationIcon(
            center.x + iconDistanceFromCenter, center.y,
            14, 7, false, 3
        );
        this.drawRotationIcon(
            center.x - iconDistanceFromCenter, center.y,
            14, 7, true, 3
        );
    }
}










/* 
 * The joystick controlling X,Y coordinates.
 */
class XyJoyStickUI extends JoyStickUI { // eslint-disable-line no-unused-vars

    constructor(params) {
        super(params);
    }

    drawBase(omitPointer) {
        super.drawBase(omitPointer);

        const iconDistanceFromCenter = this.radius * 0.75;
        const center = this.center;

        this.drawDirectionIcon(
            center.x, center.y - iconDistanceFromCenter,
            12, -Math.PI / 2
        );
        this.drawDirectionIcon(
            center.x + iconDistanceFromCenter, center.y,
            12, 0
        );
        this.drawDirectionIcon(
            center.x, center.y + iconDistanceFromCenter,
            12, Math.PI / 2
        );
        this.drawDirectionIcon(
            center.x - iconDistanceFromCenter, center.y,
            12, Math.PI
        );
    }
}
