import Toybox.Lang;
import Toybox.WatchUi;

class Delegate extends WatchUi.BehaviorDelegate {
    private var index as Number;

    public function initialize(index as Number) {
        BehaviorDelegate.initialize();
        self.index = index;
    }
}
