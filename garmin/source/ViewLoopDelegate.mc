import Toybox.Lang;
import Toybox.WatchUi;

class ViewLoopDelegate extends WatchUi.ViewLoopDelegate {

    private var viewLoop as ViewLoop;

    function initialize(viewLoop as ViewLoop) {
        ViewLoopDelegate.initialize(viewLoop);
        self.viewLoop = viewLoop;
    }

    function onNextView() {
        viewLoop.changeView(WatchUi.ViewLoop.DIRECTION_NEXT);
        return true;
    }

    function onPreviousView() {
        viewLoop.changeView(WatchUi.ViewLoop.DIRECTION_PREVIOUS);
        return true;
    }
}