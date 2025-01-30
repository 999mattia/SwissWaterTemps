import Toybox.Graphics;
import Toybox.Lang;
import Toybox.WatchUi;

class ViewLoopFactory extends WatchUi.ViewLoopFactory {
    private var entries;
    function initialize(entries) {
        ViewLoopFactory.initialize();
        self.entries = entries;
    }

    function getView(page as Number) as [View] or [View, BehaviorDelegate] {
        return [new $.ViewLoopView(page, entries[page]), new $.Delegate(page)];
    }

    function getSize() {
        return entries.size();
    }
}