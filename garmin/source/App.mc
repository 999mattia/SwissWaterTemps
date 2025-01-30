import Toybox.Application;
import Toybox.Lang;
import Toybox.WatchUi;
import Toybox.Communications;

class App extends Application.AppBase {
    var entries = [];
    public function initialize() {
        AppBase.initialize();
        makeRequests();
    }

    public function onStart(state as Dictionary?) as Void {
    }

    public function onStop(state as Dictionary?) as Void {
    }

    public function getInitialView() as [Views] or [Views, InputDelegates] {
        return [new WatchUi.ProgressBar("Loading Data...", null)];
    }

    function updateView() as Void {
        var factory = new ViewLoopFactory(entries);
        var viewLoop = new WatchUi.ViewLoop(factory, {
            :page => 0,
            :wrap => true,
            :color => Graphics.COLOR_BLUE
        });
        
        WatchUi.switchToView(viewLoop, new ViewLoopDelegate(viewLoop), WatchUi.SLIDE_UP);
    }

    function onReceive(responseCode as Number, data as Dictionary?) as Void {
        if (responseCode == 200) {
                var lakeTemperatures = data.get("lakeTemperatures") as Array;

                var riverTemperatures = data.get("riverTemperatures") as Array;

                for (var i = 0; i < lakeTemperatures.size(); i++) {
                    var entry = lakeTemperatures[i] as Entry;
                    entries.add(entry);    
                }

                for (var i = 0; i < riverTemperatures.size(); i++) {
                    var entry = riverTemperatures[i] as Entry;
                    entries.add(entry);
                }

                System.println(entries);
                updateView();
        } else {
            System.println("Error: Response Code " + responseCode);
        }
    }

    function makeRequests() as Void {
            var url = "https://swisswatertemps.mattiag.ch/api/temperatures";

            var params = {};

            var options = {
                :method => Communications.HTTP_REQUEST_METHOD_GET,
                :headers => {
                    "Content-Type" => Communications.REQUEST_CONTENT_TYPE_JSON
                },
                :responseType => Communications.HTTP_RESPONSE_CONTENT_TYPE_JSON
            };

            Communications.makeWebRequest(url, params, options, method(:onReceive));
    }
}

class Entry {
    public var name;
    public var temperature;

    public function initialize(name as String, temperature as Number) {
        self.name = name;
        self.temperature = temperature;
    }
}
