import Toybox.Graphics;
import Toybox.Lang;
import Toybox.WatchUi;

class ViewLoopView extends WatchUi.View {
    private var entry;

    private const COLOR = Graphics.COLOR_BLACK;

    public function initialize(index as Number, entry as Entry) {
        System.println(entry);
        View.initialize();
        self.entry = entry;
    }

    public function onUpdate(dc as Dc) as Void {
        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_BLACK);
        dc.clear();

        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_BLACK);
        dc.fillRectangle(0, 0, dc.getWidth(), dc.getHeight());

        dc.setColor(COLOR, Graphics.COLOR_TRANSPARENT);
        dc.drawText((dc.getWidth() / 2), (dc.getHeight() / 4), 
                     Graphics.FONT_SMALL, entry.get("name"), 
                     Graphics.TEXT_JUSTIFY_CENTER);

        dc.drawText((dc.getWidth() / 2), (dc.getHeight() / 2.5), 
                     Graphics.FONT_LARGE, entry.get("temperature").format("%.1f")+"°", 
                     Graphics.TEXT_JUSTIFY_CENTER);
    }
}
